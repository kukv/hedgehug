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

const (
	valuesPath = "/api/stores/{store}/workspaces/{workspace}/lists/{list}/values"
	wsPath     = "/api/stores/{store}/workspaces/{workspace}"
)

// AddWorkspaceValue は、値の追加の本文。
type AddWorkspaceValue struct {
	Name string `json:"name" pattern:"^[^\\r\\n]*$" doc:"1 行。改行は入れられない"`
}

// RenameWorkspaceValue は、値の名前の変更の本文。
type RenameWorkspaceValue struct {
	Name string `json:"name" pattern:"^[^\\r\\n]*$" doc:"1 行。改行は入れられない"`
}

// SetWorkspaceValueColor は、値の色の変更の本文。
type SetWorkspaceValueColor struct {
	Color string `json:"color" enum:"gray,slate,brown,red,rose,pink,magenta,purple,indigo,blue,sky,cyan,teal,green,olive,amber,yellow,orange"`
}

// SetWorkspaceStatusCategory は、ステータスの分類の変更の本文。
type SetWorkspaceStatusCategory struct {
	Category string `json:"category" enum:"todo,doing,done,canceled"`
}

// SetWorkspaceValueRef は、初期値・分類の既定にする値を指す本文。
type SetWorkspaceValueRef struct {
	Value string `json:"value"`
}

type addValueInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	List      string `path:"list"`
	Body      AddWorkspaceValue
}

type renameValueInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	List      string `path:"list"`
	Value     string `path:"value"`
	Body      RenameWorkspaceValue
}

type colorValueInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	List      string `path:"list"`
	Value     string `path:"value"`
	Body      SetWorkspaceValueColor
}

type moveValueInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	List      string `path:"list"`
	Value     string `path:"value"`
	Body      MoveWorkspaceItem
}

type deleteValueInput struct {
	Store       string `path:"store"`
	Workspace   string `path:"workspace"`
	List        string `path:"list"`
	Value       string `path:"value"`
	Replacement string `query:"replacement" doc:"代わりに使う値の ID。省略すると、使っているアイテムは、ステータス・優先度では初期値に、単一選択では「なし」になる。関連システムと複数選択では指定できず(404)、使っているアイテムから値が外れる"`
}

type categoryInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Value     string `path:"value"`
	Body      SetWorkspaceStatusCategory
}

type valueRefInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Body      SetWorkspaceValueRef
}

type categoryDefaultInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Category  string `path:"category" enum:"todo,doing,done,canceled"`
	Body      SetWorkspaceValueRef
}

type valueOutput struct{ Body WorkspaceValue }

func registerValues(api huma.API, svc appworkspaces.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "add-workspace-value", Method: http.MethodPost, Path: valuesPath,
		Summary: "値を追加", DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *addValueInput) (*valueOutput, error) {
		v, err := svc.AddValue(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), workspace.ParseList(in.List), in.Body.Name)
		if err != nil {
			return nil, apierr.From(err)
		}
		return &valueOutput{Body: toValues([]workspace.Value{v})[0]}, nil
	})
	registerValueEdits(api, svc)
	registerValueMarks(api, svc)
}

func registerValueEdits(api huma.API, svc appworkspaces.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "rename-workspace-value", Method: http.MethodPut, Path: valuesPath + "/{value}/name",
		Summary: "値の名前を変更", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *renameValueInput) (*struct{}, error) {
		return nil, apierr.From(svc.RenameValue(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace),
			workspace.ParseList(in.List), change.ID(in.Value), in.Body.Name))
	})
	huma.Register(api, huma.Operation{
		OperationID: "set-workspace-value-color", Method: http.MethodPut, Path: valuesPath + "/{value}/color",
		Summary: "値の色を変更", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *colorValueInput) (*struct{}, error) {
		return nil, apierr.From(svc.SetValueColor(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace),
			workspace.ParseList(in.List), change.ID(in.Value), workspace.Color(in.Body.Color)))
	})
	huma.Register(api, huma.Operation{
		OperationID: "move-workspace-value", Method: http.MethodPost, Path: valuesPath + "/{value}/move",
		Summary: "値を上へ・下へ移動", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *moveValueInput) (*struct{}, error) {
		return nil, apierr.From(svc.MoveValue(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace),
			workspace.ParseList(in.List), change.ID(in.Value), in.Body.Direction == "up"))
	})
	huma.Register(api, huma.Operation{
		OperationID: "delete-workspace-value", Method: http.MethodDelete, Path: valuesPath + "/{value}",
		Summary: "値を削除", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *deleteValueInput) (*struct{}, error) {
		return nil, apierr.From(svc.DeleteValue(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace),
			workspace.ParseList(in.List), change.ID(in.Value), change.ID(in.Replacement)))
	})
}

func registerValueMarks(api huma.API, svc appworkspaces.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "set-workspace-status-category", Method: http.MethodPut, Path: wsPath + "/statuses/{value}/category",
		Summary: "ステータスの分類を変更", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *categoryInput) (*struct{}, error) {
		return nil, apierr.From(svc.SetStatusCategory(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace),
			change.ID(in.Value), workspace.Category(in.Body.Category)))
	})
	huma.Register(api, huma.Operation{
		OperationID: "set-workspace-initial-status", Method: http.MethodPut, Path: wsPath + "/initial-status",
		Summary: "初期ステータスを変更", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *valueRefInput) (*struct{}, error) {
		return nil, apierr.From(svc.SetInitialStatus(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), change.ID(in.Body.Value)))
	})
	huma.Register(api, huma.Operation{
		OperationID: "set-workspace-initial-priority", Method: http.MethodPut, Path: wsPath + "/initial-priority",
		Summary: "初期優先度を変更", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *valueRefInput) (*struct{}, error) {
		return nil, apierr.From(svc.SetInitialPriority(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), change.ID(in.Body.Value)))
	})
	huma.Register(api, huma.Operation{
		OperationID: "set-workspace-category-default", Method: http.MethodPut, Path: wsPath + "/category-defaults/{category}",
		Summary: "分類の既定のステータスを変更", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *categoryDefaultInput) (*struct{}, error) {
		return nil, apierr.From(svc.SetCategoryDefault(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace),
			workspace.Category(in.Category), change.ID(in.Body.Value)))
	})
}
