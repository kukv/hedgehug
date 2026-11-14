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

// BacklogRow は、バックログの画面の枠の 1 行。
type BacklogRow struct {
	PlanningRow
	Priority string `json:"priority" doc:"作業項目の順位"`
	Project  string `json:"project" doc:"所属プロジェクトの内部 ID"`
}

// ActiveSprint と同じく、スプリントの枠だけ sprint を持つ。バックログの枠は null。
type backlogSprint struct {
	_ struct{} `nullable:"true"`
	Sprint
}

// BacklogFrame は、バックログの画面の枠。sprint が null ならバックログの枠。
type BacklogFrame struct {
	Sprint *backlogSprint `json:"sprint"`
	Total  SprintTotals   `json:"total"`
	Rows   []BacklogRow   `json:"rows" nullable:"false"`
}

// Backlog は、バックログの画面の枠の一覧。
type Backlog struct {
	Frames []BacklogFrame `json:"frames" nullable:"false"`
}

// PlaceItem は、アイテムを置く位置の本文。
type PlaceItem struct {
	Sprint *string `json:"sprint" doc:"移動先の枠のスプリントの内部 ID。null はバックログ"`
	Anchor *string `json:"anchor" doc:"基準のアイテムの内部 ID。null は枠の末尾"`
	After  bool    `json:"after" doc:"true は基準の直後、false は直前"`
}

type backlogOutput struct{ Body Backlog }

type placeItemInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  PlaceItem
}

func toBacklog(fs []appplanning.BacklogFrame) Backlog {
	out := Backlog{Frames: []BacklogFrame{}}
	for _, f := range fs {
		bf := BacklogFrame{Total: totals(f.Total), Rows: []BacklogRow{}}
		if f.Sprint != nil {
			bf.Sprint = &backlogSprint{Sprint: toSprint(*f.Sprint)}
		}
		for _, r := range f.Rows {
			row := toRows([]appplanning.Row{r.Row})[0]
			bf.Rows = append(bf.Rows, BacklogRow{PlanningRow: row, Priority: string(r.Priority), Project: string(r.Project)})
		}
		out.Frames = append(out.Frames, bf)
	}
	return out
}

func registerBacklog(api huma.API, svc appplanning.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "get-backlog", Method: http.MethodGet, Path: base + "/backlog",
		Summary: "バックログの画面の枠(スプリントの枠とバックログの枠)",
	},
		func(ctx context.Context, in *wsInput) (*backlogOutput, error) {
			store, ws := wsOf(in.Store, in.Workspace)
			fs, err := svc.Backlog(ctx, store, ws)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &backlogOutput{Body: toBacklog(fs)}, nil
		})
	huma.Register(api, huma.Operation{
		OperationID: "place-item", Method: http.MethodPost, Path: "/api/stores/{store}/items/{item}/plan-position",
		Summary: "アイテムを枠の中の位置に置く", DefaultStatus: http.StatusNoContent,
	},
		func(ctx context.Context, in *placeItemInput) (*struct{}, error) {
			// item.ID は change.ID と同じ型なので、基準も ref で nil か ID にできる。
			return nil, apierr.From(svc.PlaceItem(ctx, port.StoreID(in.Store), item.ID(in.Item), ref(in.Body.Sprint), ref(in.Body.Anchor), in.Body.After))
		})
}
