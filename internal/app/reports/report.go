package reports

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Period は、累積フロー図の期間(YYYY-MM-DD)。両方空なら直近 21 日(今日の 20 日前〜今日。F3-11)。
type Period struct{ From, To string }

// EstimateRow は、見積もりと実績の表の 1 行。Display は表示 ID(HH-12)。
type EstimateRow struct {
	planning.EstimateRow
	Display string
}

// Report は、レポートの全部の数字。Selection は読むときに直した選択(F3-5)、Unit は実際に数えた単位で、
// Fallback は件数に切り替えたか(F3-4)。Burn はバーンダウンのスプリント(未選択なら nil)。Period は使った期間。
type Report struct {
	Selection     planning.Selection
	Unit          planning.Unit
	Fallback      bool
	Sprints       []planning.Sprint
	Burn          *planning.Sprint
	Burndown      planning.Burndown
	Velocity      []planning.VelocityBar
	Average       *float64
	Period        Period
	Layers        []planning.Layer
	Flow          []planning.FlowDay
	Cycle, Lead   planning.Durations
	Estimates     []EstimateRow
	EstimateTotal planning.EstimateTotal
}

// Report は、レポートを読む。期間が合わなければ CheckPeriod の断り。
func (s Service) Report(ctx context.Context, store port.StoreID, ws workspace.ID, p Period) (Report, error) {
	sn, err := s.load(ctx, store, ws)
	if err != nil {
		return Report{}, err
	}
	now := s.Clock.Now()
	today := planning.Today(now)
	if p == (Period{}) {
		p = Period{From: planning.AddDays(today, -20), To: today}
	}
	if err := planning.CheckPeriod(p.From, p.To, today); err != nil {
		return Report{}, err
	}
	unit, fallback := planning.EffectiveUnit(sn.its, ws, sn.sel.Unit)
	r := Report{Selection: sn.sel, Unit: unit, Fallback: fallback, Sprints: planning.ReportSprints(sn.live), Period: p}
	for i := range r.Sprints {
		if r.Sprints[i].ID == sn.sel.Sprint {
			r.Burn = &r.Sprints[i]
			r.Burndown = planning.Burn(sn.works, ws, r.Sprints[i], sn.st, unit, now)
		}
	}
	r.Velocity, r.Average = planning.Velocity(sn.works, ws, sn.live, sn.st, unit, now)
	r.Layers, r.Flow = planning.Flow(sn.works, ws, sn.st, p.From, p.To, sn.sel.Layers)
	r.Cycle, r.Lead = planning.Cycle(sn.works, ws, sn.st)
	rows, total := planning.Estimates(sn.works, ws, sn.sel.Estimate)
	r.EstimateTotal = total
	for _, row := range rows {
		r.Estimates = append(r.Estimates, EstimateRow{EstimateRow: row, Display: row.Item.DisplayID(sn.ws.Prefix)})
	}
	return r, nil
}
