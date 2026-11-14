package planning

import (
	"math"
	"slices"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/worklog"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// EstimateRow は、見積もりと実績の表の 1 行。時間は 100 分の 1 時間の整数(F3-14)。
// PerPoint は 1 ポイントあたりの時間(Round1 済み。ポイントがないか 0 なら nil)。
type EstimateRow struct {
	Item     item.Item
	Points   *float64
	Estimate *int
	Worked   int
	Diff     *int
	PerPoint *float64
}

// EstimateTotal は、表の合計の行。該当する行がなければ nil。
type EstimateTotal struct {
	Points   *float64
	Estimate *int
	Worked   int
	Diff     *int
}

// Estimates は、見積もりと実績の行と合計(05「見積もりと実績」、F3-14)。
// sprint が空なら「すべて」、あればいまそのスプリントに入っているものだけ。行は表示 ID の番号の順。
func Estimates(works []Work, ws workspace.ID, sprint change.ID) ([]EstimateRow, EstimateTotal) {
	var rows []EstimateRow
	for _, w := range works {
		if !counted(w.Item, ws) || (sprint != "" && w.Item.Sprint != sprint) {
			continue
		}
		if r, ok := estimateRow(w); ok {
			rows = append(rows, r)
		}
	}
	slices.SortFunc(rows, func(a, b EstimateRow) int { return a.Item.Place.Number - b.Item.Place.Number })
	return rows, estimateTotal(rows)
}

// estimateRow は、ポイントか見積もり時間があり、作業時間の記録が 1 件以上ある作業項目の行。
func estimateRow(w Work) (EstimateRow, bool) {
	it := w.Item
	entries := worklog.FromLog(w.Log)
	if (it.Points == nil && it.Estimate == nil) || len(entries) == 0 {
		return EstimateRow{}, false
	}
	r := EstimateRow{Item: it, Points: it.Points}
	for _, e := range entries {
		r.Worked += e.Hundredths
	}
	if it.Estimate != nil {
		est := int(math.Round(*it.Estimate * 100))
		diff := r.Worked - est
		r.Estimate, r.Diff = &est, &diff
	}
	if it.Points != nil && *it.Points > 0 {
		per := Round1(float64(r.Worked) / 100 / *it.Points)
		r.PerPoint = &per
	}
	return r, true
}

// estimateTotal は、ポイントと見積もり時間は値のある行の合計(なければ nil)、作業時間はすべての行の合計、
// 差は見積もり時間のある行の差の合計(なければ nil)。
func estimateTotal(rows []EstimateRow) EstimateTotal {
	var t EstimateTotal
	for _, r := range rows {
		t.Worked += r.Worked
		if r.Points != nil {
			p := *r.Points
			if t.Points != nil {
				p += *t.Points
			}
			t.Points = &p
		}
		if r.Estimate != nil {
			e, d := *r.Estimate, *r.Diff
			if t.Estimate != nil {
				e, d = e+*t.Estimate, d+*t.Diff
			}
			t.Estimate, t.Diff = &e, &d
		}
	}
	return t
}
