package items

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appitems "github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	appwatching "github.com/kukv/hedgehug/internal/app/watching"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// SetWatch は、ウォッチの入り切りの本文。
type SetWatch struct {
	On bool `json:"on" doc:"あなたがウォッチ中にするか"`
}

type watchInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  SetWatch
}

// RegisterWatch は、ウォッチの API を登録する。ウォッチを書くのは watching、返すアイテムは items が読む。
func RegisterWatch(api huma.API, svc appitems.Service, w appwatching.Service) {
	huma.Register(api, huma.Operation{OperationID: "set-watch", Method: http.MethodPut, Path: "/api/stores/{store}/items/{item}/watch", Summary: "あなたのウォッチを入り切りする"},
		func(ctx context.Context, in *watchInput) (*oneOutput, error) {
			store, id := port.StoreID(in.Store), item.ID(in.Item)
			if err := w.Set(ctx, store, id, in.Body.On); err != nil {
				return nil, apierr.From(err)
			}
			return one(svc.Get(ctx, store, id))
		})
}
