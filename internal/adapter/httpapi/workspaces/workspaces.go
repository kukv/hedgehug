// Package workspaces は、ワークスペースの設定の API(仕様 03)を持つ。
package workspaces

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	"github.com/kukv/hedgehug/internal/app/port"
	appworkspaces "github.com/kukv/hedgehug/internal/app/workspaces"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// パスの項目は、構造体に埋め込まず入力ごとに直接持つ(埋め込むと huma がパスの値を入れない)。
type storePath struct {
	Store string `path:"store"`
}

type workspacePath struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
}

type listOutput struct{ Body []WorkspaceSummary }

type getOutput struct{ Body WorkspaceDetail }

type oneOutput struct{ Body WorkspaceSummary }

// CreateWorkspace は、ワークスペースの作成の本文。
type CreateWorkspace struct {
	Name   string `json:"name" pattern:"^[^\\r\\n]*$" doc:"1 行。改行は入れられない"`
	Prefix string `json:"prefix"`
}

type createInput struct {
	Store string `path:"store"`
	Body  CreateWorkspace
}

// RenameWorkspace は、名前の変更の本文。
type RenameWorkspace struct {
	Name string `json:"name" pattern:"^[^\\r\\n]*$" doc:"1 行。改行は入れられない"`
}

type renameInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Body      RenameWorkspace
}

// ChangeWorkspacePrefix は、接頭辞の変更の本文。
type ChangeWorkspacePrefix struct {
	Prefix string `json:"prefix"`
}

type prefixInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Body      ChangeWorkspacePrefix
}

// Register は、ワークスペースの API を登録する。
func Register(api huma.API, svc appworkspaces.Service) {
	registerRead(api, svc)
	registerWrite(api, svc)
	registerProperties(api, svc)
	registerValues(api, svc)
	registerUsage(api, svc)
	registerDelete(api, svc)
	registerTemplates(api, svc)
}

func registerRead(api huma.API, svc appworkspaces.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "list-workspaces", Method: http.MethodGet, Path: "/api/stores/{store}/workspaces",
		Summary: "Store のワークスペースの一覧",
	}, func(ctx context.Context, in *storePath) (*listOutput, error) {
		list, err := svc.List(ctx, port.StoreID(in.Store))
		if err != nil {
			return nil, apierr.From(err)
		}
		out := &listOutput{Body: []WorkspaceSummary{}}
		for _, w := range list {
			out.Body = append(out.Body, toSummary(w.Workspace, w.OldPrefixes))
		}
		return out, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "get-workspace", Method: http.MethodGet, Path: "/api/stores/{store}/workspaces/{workspace}",
		Summary: "ワークスペースと設定",
	}, func(ctx context.Context, in *workspacePath) (*getOutput, error) {
		got, err := svc.Get(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace))
		if err != nil {
			return nil, apierr.From(err)
		}
		settings := toSettings(got.Settings)
		settings.ProjectRules = toProjectRules(got.ProjectRules)
		return &getOutput{Body: WorkspaceDetail{
			WorkspaceSummary: toSummary(got.Workspace, got.OldPrefixes), Settings: settings,
		}}, nil
	})
}

func registerWrite(api huma.API, svc appworkspaces.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "create-workspace", Method: http.MethodPost, Path: "/api/stores/{store}/workspaces",
		Summary: "ワークスペースを作成", DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createInput) (*oneOutput, error) {
		w, err := svc.Create(ctx, port.StoreID(in.Store), in.Body.Name, in.Body.Prefix)
		if err != nil {
			return nil, apierr.From(err)
		}
		return &oneOutput{Body: toSummary(w, nil)}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "rename-workspace", Method: http.MethodPut, Path: "/api/stores/{store}/workspaces/{workspace}/name",
		Summary: "ワークスペースの名前を変更", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *renameInput) (*struct{}, error) {
		return nil, apierr.From(svc.Rename(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), in.Body.Name))
	})
	huma.Register(api, huma.Operation{
		OperationID: "change-workspace-prefix", Method: http.MethodPut, Path: "/api/stores/{store}/workspaces/{workspace}/prefix",
		Summary: "接頭辞を変更", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *prefixInput) (*struct{}, error) {
		return nil, apierr.From(svc.ChangePrefix(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), in.Body.Prefix))
	})
}
