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

// ChangeKind は、種類の変更の本文。Epic と Step には変えられない。
type ChangeKind struct {
	Kind string `json:"kind" enum:"Story,Task,Bug"`
}

type changeKindInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  ChangeKind
}

func registerKind(api huma.API, svc appitems.Service) {
	huma.Register(api, huma.Operation{OperationID: "change-item-kind", Method: http.MethodPut, Path: "/api/stores/{store}/items/{item}/kind", Summary: "アイテムの種類を変える", DefaultStatus: http.StatusNoContent},
		func(ctx context.Context, in *changeKindInput) (*struct{}, error) {
			if err := svc.ChangeKind(ctx, port.StoreID(in.Store), item.ID(in.Item), item.Kind(in.Body.Kind)); err != nil {
				return nil, apierr.From(err)
			}
			return nil, nil //nolint:nilnil // 204 は本文なし
		})
}
