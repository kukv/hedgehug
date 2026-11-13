package notes

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	"github.com/kukv/hedgehug/internal/app/port"
	appworklogs "github.com/kukv/hedgehug/internal/app/worklogs"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/worklog"
)

// Worklog は、API が返す作業時間の記録。Hours は「小数第 2 位まで・末尾の 0 を省いた」表記(「 時間」は付けない)。
type Worklog struct {
	ID     string `json:"id"`
	By     string `json:"by" doc:"記録した人のログイン名"`
	ByName string `json:"byName" doc:"記録した人の表示名。メンバーにいなければ「不明なユーザー」"`
	Date   string `json:"date" doc:"記録した日(YYYY-MM-DD)"`
	Hours  string `json:"hours" doc:"1.25 のような表記"`
	Memo   string `json:"memo"`
}

// WorklogInput は、作業時間の記録の本文。時間は文字列で受け取り、サーバーが検査する(A4a-26)。
type WorklogInput struct {
	Hours string `json:"hours" doc:"時間。小数第 2 位まで"`
	Memo  string `json:"memo,omitempty"`
}

type worklogOutput struct{ Body Worklog }

type worklogsOutput struct{ Body []Worklog }

type worklogsRef struct {
	Store string `path:"store"`
	Item  string `path:"item"`
}

type addWorklog struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  WorklogInput
}

type worklogRef struct {
	Store   string `path:"store"`
	Item    string `path:"item"`
	Worklog string `path:"worklog"`
}

func toWorklog(v appworklogs.View) Worklog {
	return Worklog{ID: string(v.ID), By: v.By, ByName: v.ByName, Date: v.Date, Hours: worklog.Format(v.Hundredths), Memo: v.Memo}
}

// registerWorklogs は、アイテムの作業時間の API を登録する。
func registerWorklogs(api huma.API, svc appworklogs.Service) {
	const base = "/api/stores/{store}/items/{item}/worklogs"
	huma.Register(api, huma.Operation{OperationID: "list-worklogs", Method: http.MethodGet, Path: base, Summary: "アイテムの作業時間の記録を古い順に返す"},
		func(ctx context.Context, in *worklogsRef) (*worklogsOutput, error) {
			vs, err := svc.List(ctx, port.StoreID(in.Store), item.ID(in.Item))
			if err != nil {
				return nil, apierr.From(err)
			}
			out := &worklogsOutput{Body: []Worklog{}}
			for _, v := range vs {
				out.Body = append(out.Body, toWorklog(v))
			}
			return out, nil
		})
	huma.Register(api, huma.Operation{OperationID: "add-worklog", Method: http.MethodPost, Path: base, Summary: "作業時間を記録する"},
		func(ctx context.Context, in *addWorklog) (*worklogOutput, error) {
			v, err := svc.Add(ctx, port.StoreID(in.Store), item.ID(in.Item), in.Body.Hours, in.Body.Memo)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &worklogOutput{Body: toWorklog(v)}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "delete-worklog", Method: http.MethodDelete, Path: base + "/{worklog}", Summary: "作業時間の記録を削除する", DefaultStatus: http.StatusNoContent},
		func(ctx context.Context, in *worklogRef) (*struct{}, error) {
			return nil, apierr.From(svc.Delete(ctx, port.StoreID(in.Store), item.ID(in.Item), change.ID(in.Worklog)))
		})
}
