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

// Duplicated は、複製の応答。
type Duplicated struct {
	ID string `json:"id" doc:"新しいアイテムの内部 ID"`
}

type duplicatedOutput struct{ Body Duplicated }

func registerDuplicate(api huma.API, svc appitems.Service) {
	huma.Register(api, huma.Operation{OperationID: "duplicate-item", Method: http.MethodPost, Path: "/api/stores/{store}/items/{item}/duplicate", Summary: "アイテムを複製する"},
		func(ctx context.Context, in *getInput) (*duplicatedOutput, error) {
			id, err := svc.Duplicate(ctx, port.StoreID(in.Store), item.ID(in.Item))
			if err != nil {
				return nil, apierr.From(err)
			}
			return &duplicatedOutput{Body: Duplicated{ID: string(id)}}, nil
		})
}
