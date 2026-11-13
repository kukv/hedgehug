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

// ReorderItem は、並べ替えの本文。Anchor は、同じ親の削除されていない兄弟の内部 ID。
type ReorderItem struct {
	Anchor string `json:"anchor" minLength:"1" doc:"基準にする兄弟の内部 ID。絞り込み中は、見えている兄弟"`
	After  bool   `json:"after" doc:"true なら基準の直後、false なら直前に置く"`
}

type reorderInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  ReorderItem
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
	huma.Register(api, huma.Operation{OperationID: "reorder-item", Method: http.MethodPost, Path: "/api/stores/{store}/items/{item}/reorder", Summary: "アイテムを、同じ親の兄弟の直前か直後に置く", DefaultStatus: http.StatusNoContent},
		func(ctx context.Context, in *reorderInput) (*struct{}, error) {
			if err := svc.Reorder(ctx, port.StoreID(in.Store), item.ID(in.Item), item.ID(in.Body.Anchor), in.Body.After); err != nil {
				return nil, apierr.From(err)
			}
			return nil, nil //nolint:nilnil // 204 は本文なし
		})
}
