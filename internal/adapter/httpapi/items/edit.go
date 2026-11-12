package items

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appitems "github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// RenameItem は、タイトルの変更の本文。
type RenameItem struct {
	Title string `json:"title"`
}

// AddStep は、Step の追加の本文。
type AddStep struct {
	Title string `json:"title"`
}

type renameInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  RenameItem
}

type addStepInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  AddStep
}

func registerEdit(api huma.API, svc appitems.Service) {
	huma.Register(api, huma.Operation{OperationID: "rename-item", Method: http.MethodPut, Path: "/api/stores/{store}/items/{item}/title", Summary: "アイテムのタイトルを変える"},
		func(ctx context.Context, in *renameInput) (*oneOutput, error) {
			it, err := svc.Rename(ctx, port.StoreID(in.Store), item.ID(in.Item), in.Body.Title)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &oneOutput{Body: toAPI(it)}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "add-step", Method: http.MethodPost, Path: "/api/stores/{store}/items/{item}/steps", Summary: "アイテムの子の末尾に Step を作る"},
		func(ctx context.Context, in *addStepInput) (*oneOutput, error) {
			it, err := svc.AddStep(ctx, port.StoreID(in.Store), item.ID(in.Item), in.Body.Title)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &oneOutput{Body: toAPI(it)}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "move-item-up", Method: http.MethodPost, Path: "/api/stores/{store}/items/{item}/move-up", Summary: "アイテムを直前の兄弟と入れ替える", DefaultStatus: http.StatusNoContent},
		func(ctx context.Context, in *getInput) (*struct{}, error) {
			if err := svc.MoveUp(ctx, port.StoreID(in.Store), item.ID(in.Item)); err != nil {
				return nil, apierr.From(err)
			}
			return nil, nil //nolint:nilnil // 204 は本文なし
		})
}
