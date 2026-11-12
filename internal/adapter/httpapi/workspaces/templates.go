package workspaces

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	"github.com/kukv/hedgehug/internal/app/port"
	appworkspaces "github.com/kukv/hedgehug/internal/app/workspaces"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// SetWorkspaceTransition は、遷移の制約の表のセルの本文。
type SetWorkspaceTransition struct {
	Allowed bool `json:"allowed"`
}

// SetWorkspaceTemplateDefault は、種類ごとの既定の本文。
type SetWorkspaceTemplateDefault struct {
	Template string `json:"template" doc:"テンプレートの ID。空なら「なし」"`
}

// SaveItemAsTemplate は、「テンプレートとして保存…」の本文。
type SaveItemAsTemplate struct {
	Name      string `json:"name"`
	Title     string `json:"title"`
	Structure bool   `json:"structure" doc:"子と Step を含める"`
}

type transitionInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	From      string `path:"from"`
	To        string `path:"to"`
	Body      SetWorkspaceTransition
}

type createTemplateInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Body      WorkspaceTemplateInput
}

type updateTemplateInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Template  string `path:"template"`
	Body      WorkspaceTemplateInput
}

type templatePath struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Template  string `path:"template"`
}

type templateDefaultInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Kind      string `path:"kind" enum:"Epic,Story,Task,Bug,Step"`
	Body      SetWorkspaceTemplateDefault
}

type saveAsTemplateInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  SaveItemAsTemplate
}

type templateOutput struct{ Body WorkspaceTemplate }

func registerTemplates(api huma.API, svc appworkspaces.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "set-workspace-transition", Method: http.MethodPut, Path: wsPath + "/workflow/{from}/{to}",
		Summary: "遷移の制約のセルを変更", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *transitionInput) (*struct{}, error) {
		return nil, apierr.From(svc.SetTransition(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace),
			change.ID(in.From), change.ID(in.To), in.Body.Allowed))
	})
	huma.Register(api, huma.Operation{
		OperationID: "create-workspace-template", Method: http.MethodPost, Path: wsPath + "/templates",
		Summary: "テンプレートを作成", DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createTemplateInput) (*templateOutput, error) {
		t, err := svc.CreateTemplate(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), fromTemplate(in.Body))
		if err != nil {
			return nil, apierr.From(err)
		}
		return &templateOutput{Body: toTemplate(t)}, nil
	})
	registerTemplateEdits(api, svc)
}

func registerTemplateEdits(api huma.API, svc appworkspaces.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "update-workspace-template", Method: http.MethodPut, Path: wsPath + "/templates/{template}",
		Summary: "テンプレートを編集", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *updateTemplateInput) (*struct{}, error) {
		return nil, apierr.From(svc.UpdateTemplate(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace),
			change.ID(in.Template), fromTemplate(in.Body)))
	})
	huma.Register(api, huma.Operation{
		OperationID: "delete-workspace-template", Method: http.MethodDelete, Path: wsPath + "/templates/{template}",
		Summary: "テンプレートを削除", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *templatePath) (*struct{}, error) {
		return nil, apierr.From(svc.DeleteTemplate(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), change.ID(in.Template)))
	})
	huma.Register(api, huma.Operation{
		OperationID: "set-workspace-template-default", Method: http.MethodPut, Path: wsPath + "/template-defaults/{kind}",
		Summary: "種類ごとの既定のテンプレートを変更", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *templateDefaultInput) (*struct{}, error) {
		return nil, apierr.From(svc.SetTemplateDefault(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace),
			in.Kind, change.ID(in.Body.Template)))
	})
	huma.Register(api, huma.Operation{
		OperationID: "save-item-as-template", Method: http.MethodPost, Path: "/api/stores/{store}/items/{item}/save-as-template",
		Summary: "アイテムをテンプレートとして保存", DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *saveAsTemplateInput) (*templateOutput, error) {
		t, err := svc.CreateTemplateFromItem(ctx, port.StoreID(in.Store), item.ID(in.Item), appworkspaces.FromItem{
			Name: in.Body.Name, Title: in.Body.Title, Structure: in.Body.Structure,
		})
		if err != nil {
			return nil, apierr.From(err)
		}
		return &templateOutput{Body: toTemplate(t)}, nil
	})
}
