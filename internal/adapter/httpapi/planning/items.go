package planning

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appplanning "github.com/kukv/hedgehug/internal/app/planning"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// SetItemSprint は、アイテムのスプリントの変更の本文。
type SetItemSprint struct {
	Sprint *string `json:"sprint" doc:"スプリントの内部 ID。null はバックログ"`
}

// SetItemRelease は、アイテムのリリースの変更の本文。
type SetItemRelease struct {
	Release *string `json:"release" doc:"リリースの内部 ID。null は「なし」"`
}

type itemSprintInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  SetItemSprint
}

type itemReleaseInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  SetItemRelease
}

func registerItems(api huma.API, svc appplanning.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "set-item-sprint", Method: http.MethodPut, Path: "/api/stores/{store}/items/{item}/sprint",
		Summary: "アイテムのスプリントを変える", DefaultStatus: http.StatusNoContent,
	},
		func(ctx context.Context, in *itemSprintInput) (*struct{}, error) {
			return nil, apierr.From(svc.SetSprint(ctx, port.StoreID(in.Store), item.ID(in.Item), ref(in.Body.Sprint)))
		})
	huma.Register(api, huma.Operation{
		OperationID: "set-item-release", Method: http.MethodPut, Path: "/api/stores/{store}/items/{item}/release",
		Summary: "アイテムのリリースを変える", DefaultStatus: http.StatusNoContent,
	},
		func(ctx context.Context, in *itemReleaseInput) (*struct{}, error) {
			return nil, apierr.From(svc.SetRelease(ctx, port.StoreID(in.Store), item.ID(in.Item), ref(in.Body.Release)))
		})
}
