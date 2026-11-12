package items

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appitems "github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// SetItemProperty は、プロパティの変更の本文。value は値(数は文字列のまま。決定 12)。
type SetItemProperty struct {
	Value json.RawMessage `json:"value" doc:"新しい値。null と空の文字列は値を空にする"`
}

// CheckStep は、Step のチェックの本文。
type CheckStep struct {
	Done bool `json:"done"`
}

type propertyInput struct {
	Store    string `path:"store"`
	Item     string `path:"item"`
	Property string `path:"property"`
	Body     SetItemProperty
}

type valueInput struct {
	Store    string `path:"store"`
	Item     string `path:"item"`
	Property string `path:"property"`
	Value    string `path:"value"`
}

type checkInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  CheckStep
}

func registerValues(api huma.API, svc appitems.Service) {
	huma.Register(api, huma.Operation{OperationID: "set-item-property", Method: http.MethodPut, Path: "/api/stores/{store}/items/{item}/properties/{property}", Summary: "アイテムのプロパティの値を変える"},
		func(ctx context.Context, in *propertyInput) (*oneOutput, error) {
			return one(svc.SetProperty(ctx, port.StoreID(in.Store), item.ID(in.Item), change.ID(in.Property), in.Body.Value))
		})
	huma.Register(api, huma.Operation{OperationID: "add-item-value", Method: http.MethodPost, Path: "/api/stores/{store}/items/{item}/properties/{property}/values/{value}", Summary: "関連システム・複数選択に値を足す"},
		func(ctx context.Context, in *valueInput) (*oneOutput, error) {
			return one(svc.AddValue(ctx, port.StoreID(in.Store), item.ID(in.Item), change.ID(in.Property), change.ID(in.Value)))
		})
	huma.Register(api, huma.Operation{OperationID: "remove-item-value", Method: http.MethodDelete, Path: "/api/stores/{store}/items/{item}/properties/{property}/values/{value}", Summary: "関連システム・複数選択から値を取り除く"},
		func(ctx context.Context, in *valueInput) (*oneOutput, error) {
			return one(svc.RemoveValue(ctx, port.StoreID(in.Store), item.ID(in.Item), change.ID(in.Property), change.ID(in.Value)))
		})
	huma.Register(api, huma.Operation{OperationID: "check-step", Method: http.MethodPost, Path: "/api/stores/{store}/items/{item}/check", Summary: "Step のチェックを入り切りする"},
		func(ctx context.Context, in *checkInput) (*oneOutput, error) {
			return one(svc.CheckStep(ctx, port.StoreID(in.Store), item.ID(in.Item), in.Body.Done))
		})
}

// one は、アイテムを返す。値の誤りと親の誤りは画面からは送らないので 422(決定 16)。
func one(it item.Item, err error) (*oneOutput, error) {
	if errors.Is(err, item.ErrBadInput) || errors.Is(err, appitems.ErrParentElsewhere) {
		return nil, huma.Error422UnprocessableEntity(err.Error())
	}
	if err != nil {
		return nil, apierr.From(err)
	}
	return &oneOutput{Body: toAPI(it)}, nil
}
