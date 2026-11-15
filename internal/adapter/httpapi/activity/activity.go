// Package activity は、アクティビティの API(仕様 06「アクティビティ」)。
package activity

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appactivity "github.com/kukv/hedgehug/internal/app/activity"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

type listInput struct {
	Store     string `query:"store" doc:"この Store の行だけ。空ならすべての Store"`
	Workspace string `query:"workspace" doc:"このワークスペースの行と、同じ Store の同期の行・Store 全体の行だけ。store と一緒に渡す。片方だけは 422"`
	Kind      string `query:"kind" enum:",all,user,sync,automation" doc:"種類の絞り込み。空は all"`
	Before    string `query:"before" doc:"続きを読むとき、前のページの最後の行の cursor。読めない値は 422"`
}

type pageOutput struct{ Body ActivityPage }

// Register は、アクティビティの API を登録する。
func Register(api huma.API, svc appactivity.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "list-activity", Method: http.MethodGet, Path: "/api/activity",
		Summary: "Store のコミットと同期の行を、新しい順に 300 件",
	},
		func(ctx context.Context, in *listInput) (*pageOutput, error) {
			p, err := svc.Page(ctx, appactivity.Query{
				Store: port.StoreID(in.Store), Workspace: workspace.ID(in.Workspace),
				Kind: appactivity.Kind(in.Kind), Before: in.Before,
			})
			if errors.Is(err, port.ErrInvalidOperation) {
				return nil, huma.Error422UnprocessableEntity(err.Error())
			}
			if err != nil {
				return nil, apierr.From(err)
			}
			return &pageOutput{Body: toPage(p)}, nil
		})
}
