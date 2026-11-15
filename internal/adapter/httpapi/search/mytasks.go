package search

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appsearch "github.com/kukv/hedgehug/internal/app/search"
)

// MyTasksQuery は、マイタスクの絞り込み。基本のクエリ(自分が担当・未完了・Epic 以外)に AND でつなぐ。
type MyTasksQuery struct {
	Filter string `json:"filter" doc:"絞り込みの文字列(04「検索」と同じ書き方)。空なら基本のクエリだけ。sort:・group: は無視する"`
}

// MyTasksResult は、マイタスクの結果。エラーがあれば rows は空。
type MyTasksResult struct {
	Errors []string    `json:"errors" nullable:"false" doc:"絞り込みのエラー(1 語 1 行)"`
	Rows   []SearchRow `json:"rows" nullable:"false"`
}

type myTasksInput struct{ Body MyTasksQuery }

type myTasksOutput struct{ Body MyTasksResult }

func registerMyTasks(api huma.API, svc appsearch.Service) {
	huma.Register(api, huma.Operation{OperationID: "my-tasks", Method: http.MethodPost, Path: "/api/my-tasks", Summary: "自分が担当する未完了のアイテムを、絞り込みをつないで返す"},
		func(ctx context.Context, in *myTasksInput) (*myTasksOutput, error) {
			res, err := svc.MyTasks(ctx, in.Body.Filter)
			if err != nil {
				return nil, apierr.From(err)
			}
			out := MyTasksResult{Errors: append([]string{}, res.Errors...), Rows: []SearchRow{}}
			for _, r := range res.Rows {
				out.Rows = append(out.Rows, toRow(r))
			}
			return &myTasksOutput{Body: out}, nil
		})
}
