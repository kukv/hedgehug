package notes

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	apphistory "github.com/kukv/hedgehug/internal/app/history"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	dhistory "github.com/kukv/hedgehug/internal/domain/history"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// HistoryRow は、履歴の 1 行。文はサーバーが作る(A4a-15)。
type HistoryRow struct {
	Who  string `json:"who" doc:"操作した人の表示(メンバーの名前・自動化・GitHub・同期・不明なユーザー)"`
	Text string `json:"text" doc:"変更の文(作成・タイトルを「…」に変更 など)"`
	At   string `json:"at" doc:"時刻(ミリ秒までの RFC 3339)"`
}

type itemHistoryInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
}

type projectHistoryInput struct {
	Store   string `path:"store"`
	Project string `path:"project"`
}

type historyOutput struct{ Body []HistoryRow }

func historyRows(rows []dhistory.Row) *historyOutput {
	out := &historyOutput{Body: []HistoryRow{}}
	for _, r := range rows {
		out.Body = append(out.Body, HistoryRow{Who: r.Who, Text: r.Text, At: r.At.Format(change.TimeLayout)})
	}
	return out
}

func registerHistory(api huma.API, svc apphistory.Service) {
	huma.Register(api, huma.Operation{OperationID: "get-item-history", Method: http.MethodGet, Path: "/api/stores/{store}/items/{item}/history", Summary: "アイテムの履歴を新しい順で返す"},
		func(ctx context.Context, in *itemHistoryInput) (*historyOutput, error) {
			rows, err := svc.Item(ctx, port.StoreID(in.Store), item.ID(in.Item))
			if err != nil {
				return nil, apierr.From(err)
			}
			return historyRows(rows), nil
		})
	huma.Register(api, huma.Operation{OperationID: "get-project-history", Method: http.MethodGet, Path: "/api/stores/{store}/projects/{project}/history", Summary: "プロジェクトの履歴を新しい順で返す"},
		func(ctx context.Context, in *projectHistoryInput) (*historyOutput, error) {
			rows, err := svc.Project(ctx, port.StoreID(in.Store), project.ID(in.Project))
			if err != nil {
				return nil, apierr.From(err)
			}
			return historyRows(rows), nil
		})
}
