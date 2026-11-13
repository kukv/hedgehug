// Package automations は、自動化の API(仕様 03「自動化」)。
package automations

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appautomations "github.com/kukv/hedgehug/internal/app/automations"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const base = "/api/stores/{store}/workspaces/{workspace}/automation"

type wsInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
}

type ruleInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Rule      string `path:"rule"`
}

type createInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Body      AutomationRuleInput
}

type updateInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Rule      string `path:"rule"`
	Body      AutomationRuleInput
}

type enabledInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Rule      string `path:"rule"`
	Body      SetAutomationRuleEnabled
}

type applyInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Rule  string `path:"rule"`
	Body  ApplyAutomationIgnoringWorkflow
}

type (
	viewOutput struct{ Body WorkspaceAutomation }
	ruleOutput struct{ Body AutomationRule }
)

// Register は、自動化の API を登録する。
func Register(api huma.API, svc appautomations.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "get-workspace-automation", Method: http.MethodGet, Path: base,
		Summary: "自動化のルールと最近の実行",
	},
		func(ctx context.Context, in *wsInput) (*viewOutput, error) {
			v, err := svc.List(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace))
			if err != nil {
				return nil, apierr.From(err)
			}
			return &viewOutput{Body: toView(v)}, nil
		})
	huma.Register(api, huma.Operation{
		OperationID: "create-automation-rule", Method: http.MethodPost, Path: base + "/rules",
		Summary: "ルールを作成", DefaultStatus: http.StatusCreated,
	},
		func(ctx context.Context, in *createInput) (*ruleOutput, error) {
			r, err := svc.Create(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), fromInput(in.Body))
			if err != nil {
				return nil, apierr.From(err)
			}
			return &ruleOutput{Body: toRule(r)}, nil
		})
	registerEdits(api, svc)
}

func registerEdits(api huma.API, svc appautomations.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "update-automation-rule", Method: http.MethodPut, Path: base + "/rules/{rule}",
		Summary: "ルールを編集", DefaultStatus: http.StatusNoContent,
	},
		func(ctx context.Context, in *updateInput) (*struct{}, error) {
			return nil, apierr.From(svc.Update(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), change.ID(in.Rule), fromInput(in.Body)))
		})
	huma.Register(api, huma.Operation{
		OperationID: "delete-automation-rule", Method: http.MethodDelete, Path: base + "/rules/{rule}",
		Summary: "ルールを削除", DefaultStatus: http.StatusNoContent,
	},
		func(ctx context.Context, in *ruleInput) (*struct{}, error) {
			return nil, apierr.From(svc.Delete(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), change.ID(in.Rule)))
		})
	huma.Register(api, huma.Operation{
		OperationID: "set-automation-rule-enabled", Method: http.MethodPut, Path: base + "/rules/{rule}/enabled",
		Summary: "ルールを有効・無効にする", DefaultStatus: http.StatusNoContent,
	},
		func(ctx context.Context, in *enabledInput) (*struct{}, error) {
			return nil, apierr.From(svc.SetEnabled(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), change.ID(in.Rule), in.Body.Enabled))
		})
	huma.Register(api, huma.Operation{
		OperationID: "apply-automation-ignoring-workflow", Method: http.MethodPost,
		Path: "/api/stores/{store}/items/{item}/automation/{rule}/apply", Summary: "止まった自動化の変更を、制約を無視して適用",
		DefaultStatus: http.StatusNoContent,
	},
		func(ctx context.Context, in *applyInput) (*struct{}, error) {
			return nil, apierr.From(svc.ApplyIgnoringWorkflow(ctx, port.StoreID(in.Store), item.ID(in.Item), change.ID(in.Rule), change.ID(in.Body.Status)))
		})
}
