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

// WorkspaceUsage は、アイテムの件数と、値・プロパティごとの件数。
type WorkspaceUsage struct {
	Items        int            `json:"items"`
	FirstNumbers []int          `json:"firstNumbers" nullable:"false" doc:"番号の小さい方から 3 つ"`
	Values       map[string]int `json:"values" doc:"値の ID ごとの、その値を持つアイテムの数"`
	Properties   map[string]int `json:"properties" doc:"追加プロパティの ID ごとの、値の入っているアイテムの数"`
}

type usageOutput struct{ Body WorkspaceUsage }

func toUsage(u appworkspaces.Usage) WorkspaceUsage {
	out := WorkspaceUsage{Items: u.Items, FirstNumbers: u.FirstNumbers, Values: map[string]int{}, Properties: map[string]int{}}
	if out.FirstNumbers == nil {
		out.FirstNumbers = []int{}
	}
	for id, n := range u.Values {
		out.Values[string(id)] = n
	}
	for id, n := range u.Properties {
		out.Properties[string(id)] = n
	}
	return out
}

func registerUsage(api huma.API, svc appworkspaces.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "get-workspace-usage", Method: http.MethodGet, Path: wsPath + "/usage",
		Summary: "アイテムの件数と、値・プロパティごとの件数",
	}, func(ctx context.Context, in *workspacePath) (*usageOutput, error) {
		u, err := svc.Usage(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace))
		if err != nil {
			return nil, apierr.From(err)
		}
		return &usageOutput{Body: toUsage(u)}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "move-workspace", Method: http.MethodPost, Path: wsPath + "/move",
		Summary: "ワークスペースを上へ・下へ移動", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *moveWorkspaceInput) (*struct{}, error) {
		return nil, apierr.From(svc.MoveWorkspace(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), in.Body.Direction == "up"))
	})
}

// WorkspaceDeletePreview は、ワークスペースの削除の確認ダイアログの中身。
type WorkspaceDeletePreview struct {
	Projects      int `json:"projects" doc:"削除されていないプロジェクトの数"`
	Items         int `json:"items" doc:"その下の、削除されていないアイテムの数(Epic から Step まで全部)"`
	RetentionDays int `json:"retentionDays" doc:"その Store の削除済みの保持日数"`
}

type deletePreviewOutput struct{ Body WorkspaceDeletePreview }

func registerDelete(api huma.API, svc appworkspaces.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "get-workspace-delete-preview", Method: http.MethodGet, Path: wsPath + "/delete-preview",
		Summary: "ワークスペースを削除したときに一緒に消えるプロジェクトとアイテムの数と保持日数を返す",
	}, func(ctx context.Context, in *workspacePath) (*deletePreviewOutput, error) {
		p, err := svc.DeletePreview(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace))
		if err != nil {
			return nil, apierr.From(err)
		}
		return &deletePreviewOutput{Body: WorkspaceDeletePreview{Projects: p.Projects, Items: p.Items, RetentionDays: p.RetentionDays}}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "delete-workspace", Method: http.MethodDelete, Path: wsPath,
		Summary: "ワークスペースと、その下のプロジェクト・アイテムを削除する", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *workspacePath) (*struct{}, error) {
		return nil, apierr.From(svc.Delete(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace)))
	})
}

type moveWorkspaceInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Body      MoveWorkspaceItem
}
