package workspaces

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	"github.com/kukv/hedgehug/internal/app/port"
	appworkspaces "github.com/kukv/hedgehug/internal/app/workspaces"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const propertiesPath = "/api/stores/{store}/workspaces/{workspace}/properties"

// AddWorkspaceProperty は、追加プロパティの作成の本文。
type AddWorkspaceProperty struct {
	Name  string   `json:"name" pattern:"^[^\\r\\n]*$" doc:"1 行。改行は入れられない"`
	Key   string   `json:"key"`
	Type  string   `json:"type" enum:"text,number,date,single,multi,user,check,url"`
	Kinds []string `json:"kinds" nullable:"false" doc:"使う種類。空ならすべて"`
}

// RenameWorkspaceProperty は、プロパティの名前の変更の本文。
type RenameWorkspaceProperty struct {
	Name string `json:"name" pattern:"^[^\\r\\n]*$" doc:"1 行。改行は入れられない"`
}

// SetWorkspacePropertyKinds は、プロパティの使う種類の変更の本文。
type SetWorkspacePropertyKinds struct {
	Kinds []string `json:"kinds" nullable:"false"`
}

// SetWorkspacePropertyHidden は、プロパティの表示・非表示の変更の本文。
type SetWorkspacePropertyHidden struct {
	Hidden bool `json:"hidden"`
}

// MoveWorkspaceItem は、プロパティ・値・ワークスペースの並べ替えの本文。
type MoveWorkspaceItem struct {
	Direction string `json:"direction" enum:"up,down"`
}

type addPropertyInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Body      AddWorkspaceProperty
}

type propertyPath struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Property  string `path:"property"`
}

type renamePropertyInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Property  string `path:"property"`
	Body      RenameWorkspaceProperty
}

type propertyKindsInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Property  string `path:"property"`
	Body      SetWorkspacePropertyKinds
}

type propertyHiddenInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Property  string `path:"property"`
	Body      SetWorkspacePropertyHidden
}

type movePropertyInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Property  string `path:"property"`
	Body      MoveWorkspaceItem
}

type propertyOutput struct{ Body WorkspaceProperty }

func registerProperties(api huma.API, svc appworkspaces.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "add-workspace-property", Method: http.MethodPost, Path: propertiesPath,
		Summary: "追加プロパティを作成", DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *addPropertyInput) (*propertyOutput, error) {
		p, err := svc.AddProperty(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), appworkspaces.NewProperty{
			Name: in.Body.Name, Key: in.Body.Key, Type: workspace.PropertyType(in.Body.Type), Kinds: in.Body.Kinds,
		})
		if err != nil {
			return nil, apierr.From(err)
		}
		return &propertyOutput{Body: toProperty(p)}, nil
	})
	registerPropertyEdits(api, svc)
}

func registerPropertyEdits(api huma.API, svc appworkspaces.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "rename-workspace-property", Method: http.MethodPut, Path: propertiesPath + "/{property}/name",
		Summary: "プロパティの名前を変更", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *renamePropertyInput) (*struct{}, error) {
		return nil, apierr.From(svc.RenameProperty(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), change.ID(in.Property), in.Body.Name))
	})
	huma.Register(api, huma.Operation{
		OperationID: "set-workspace-property-kinds", Method: http.MethodPut, Path: propertiesPath + "/{property}/kinds",
		Summary: "プロパティを使う種類を変更", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *propertyKindsInput) (*struct{}, error) {
		return nil, apierr.From(svc.SetPropertyKinds(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), change.ID(in.Property), in.Body.Kinds))
	})
	huma.Register(api, huma.Operation{
		OperationID: "set-workspace-property-hidden", Method: http.MethodPut, Path: propertiesPath + "/{property}/hidden",
		Summary: "プロパティの表示・非表示を変更", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *propertyHiddenInput) (*struct{}, error) {
		return nil, apierr.From(svc.SetPropertyHidden(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), change.ID(in.Property), in.Body.Hidden))
	})
	huma.Register(api, huma.Operation{
		OperationID: "move-workspace-property", Method: http.MethodPost, Path: propertiesPath + "/{property}/move",
		Summary: "プロパティを上へ・下へ移動", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *movePropertyInput) (*struct{}, error) {
		return nil, apierr.From(svc.MoveProperty(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), change.ID(in.Property), in.Body.Direction == "up"))
	})
	huma.Register(api, huma.Operation{
		OperationID: "delete-workspace-property", Method: http.MethodDelete, Path: propertiesPath + "/{property}",
		Summary: "プロパティを削除", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *propertyPath) (*struct{}, error) {
		return nil, apierr.From(svc.DeleteProperty(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), change.ID(in.Property)))
	})
}
