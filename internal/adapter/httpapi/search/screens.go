package search

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appsearch "github.com/kukv/hedgehug/internal/app/search"
)

// maxScreenBody は、画面の状態の大きさの上限(E1-16。バイト数)。
const maxScreenBody = 64 * 1024

// ScreenState は、画面の状態。サーバーは中身を見ない(画面ごとの JSON の文字列)。
type ScreenState struct {
	Body *string `json:"body" required:"true" nullable:"true" doc:"保存した文字列。まだなければ null"`
}

// ScreenStateBody は、画面の状態を置く本文。
type ScreenStateBody struct {
	Body string `json:"body" doc:"画面の状態の JSON の文字列。64 KiB まで"`
}

type screenGetInput struct {
	Screen string `path:"screen" pattern:"^[a-z0-9:_/-]{1,200}$" doc:"画面の鍵(search など)"`
}

type screenGetOutput struct{ Body ScreenState }

type screenPutInput struct {
	Screen string `path:"screen" pattern:"^[a-z0-9:_/-]{1,200}$" doc:"画面の鍵(search など)"`
	Body   ScreenStateBody
}

func registerScreens(api huma.API, screens appsearch.ScreenStates) {
	huma.Register(api, huma.Operation{OperationID: "screen-state-get", Method: http.MethodGet, Path: "/api/screen-states/{screen}", Summary: "画面の状態を返す"},
		func(ctx context.Context, in *screenGetInput) (*screenGetOutput, error) {
			body, ok, err := screens.Get(ctx, in.Screen)
			if err != nil {
				return nil, apierr.From(err)
			}
			out := &screenGetOutput{}
			if ok {
				out.Body.Body = &body
			}
			return out, nil
		})
	huma.Register(api, huma.Operation{OperationID: "screen-state-put", Method: http.MethodPut, Path: "/api/screen-states/{screen}", Summary: "画面の状態を置く", DefaultStatus: http.StatusNoContent},
		func(ctx context.Context, in *screenPutInput) (*struct{}, error) {
			if len(in.Body.Body) > maxScreenBody {
				return nil, huma.Error422UnprocessableEntity("画面の状態が大きすぎます(64 KiB まで)")
			}
			if err := screens.Put(ctx, in.Screen, in.Body.Body); err != nil {
				return nil, apierr.From(err)
			}
			return nil, nil
		})
}
