// Package reports は、レポートと実行中のスプリントの数字の読み取り、レポートの選択の保存の API(仕様 05「レポート」)。
package reports

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	"github.com/kukv/hedgehug/internal/app/port"
	appreports "github.com/kukv/hedgehug/internal/app/reports"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const base = "/api/stores/{store}/workspaces/{workspace}"

type wsInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
}

type reportInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	From      string `query:"from" pattern:"^(\\d{4}-\\d{2}-\\d{2})?$" doc:"累積フロー図の開始日(YYYY-MM-DD)。to と両方なければ直近 21 日"`
	To        string `query:"to" pattern:"^(\\d{4}-\\d{2}-\\d{2})?$" doc:"累積フロー図の終了日(YYYY-MM-DD)。今日以前"`
}

type selectionInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Body      ReportSelectionPatch
}

type (
	viewOutput   struct{ Body ReportView }
	activeOutput struct{ Body ReportActive }
)

func (p ReportSelectionPatch) app() appreports.Patch {
	var out appreports.Patch
	if p.Unit != nil {
		u := planning.Unit(*p.Unit)
		out.Unit = &u
	}
	if p.Sprint != nil {
		id := change.ID(*p.Sprint)
		out.Sprint = &id
	}
	if p.Layers != nil {
		l := planning.Layers(*p.Layers)
		out.Layers = &l
	}
	if p.Estimate != nil {
		id := change.ID(*p.Estimate)
		out.Estimate = &id
	}
	return out
}

// Register は、レポートの API を登録する。
func Register(api huma.API, svc appreports.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "get-report", Method: http.MethodGet, Path: base + "/reports",
		Summary: "レポートの数字",
	},
		func(ctx context.Context, in *reportInput) (*viewOutput, error) {
			if err := apierr.CheckDates(in.From, in.To); err != nil {
				return nil, err
			}
			r, err := svc.Report(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), appreports.Period{From: in.From, To: in.To})
			if err != nil {
				return nil, apierr.From(err)
			}
			return &viewOutput{Body: toView(r)}, nil
		})
	huma.Register(api, huma.Operation{
		OperationID: "get-report-active", Method: http.MethodGet, Path: base + "/reports/active",
		Summary: "実行中のスプリントの小さいバーンダウン・途中で追加した量・進捗",
	},
		func(ctx context.Context, in *wsInput) (*activeOutput, error) {
			a, err := svc.Active(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace))
			if err != nil {
				return nil, apierr.From(err)
			}
			return &activeOutput{Body: toActive(a)}, nil
		})
	huma.Register(api, huma.Operation{
		OperationID: "put-report-selection", Method: http.MethodPut, Path: base + "/reports/selection",
		Summary: "レポートの選択を保存(送った項目だけ変える。Store には書かない)", DefaultStatus: http.StatusNoContent,
	},
		func(ctx context.Context, in *selectionInput) (*struct{}, error) {
			return nil, apierr.From(svc.Select(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), in.Body.app()))
		})
}
