package reports

import (
	appreports "github.com/kukv/hedgehug/internal/app/reports"
	"github.com/kukv/hedgehug/internal/domain/planning"
)

// ReportSelectionPatch は、レポートの選択の変更の本文。どれも省略でき、省略した項目は変えない。
type ReportSelectionPatch struct {
	Unit     *string `json:"unit,omitempty" doc:"points か count。違う値は 400(enum にすると 422 になるので付けない)"`
	Sprint   *string `json:"sprint,omitempty" doc:"バーンダウンのスプリントの内部 ID。空は選んでいない"`
	Layers   *string `json:"layers,omitempty" doc:"category か status。違う値は 400"`
	Estimate *string `json:"estimate,omitempty" doc:"見積もりと実績の対象のスプリントの内部 ID。空はすべて"`
}

// ReportSelection は、読むときに直したレポートの選択。
type ReportSelection struct {
	Unit     string `json:"unit" enum:"points,count"`
	Sprint   string `json:"sprint" doc:"バーンダウンのスプリントの内部 ID。空は未選択"`
	Layers   string `json:"layers" enum:"category,status"`
	Estimate string `json:"estimate" doc:"見積もりと実績の対象のスプリントの内部 ID。空はすべて"`
}

// ReportTotals は、ポイントと件数の合計。
type ReportTotals struct {
	Points float64 `json:"points"`
	Count  int     `json:"count"`
}

// ReportConfirmed は、完了で確定した計画(committed)と実績(completed)。
type ReportConfirmed struct {
	_         struct{}     `nullable:"true"`
	Committed ReportTotals `json:"committed"`
	Completed ReportTotals `json:"completed"`
}

// ReportSprint は、レポートのスプリント。項目は計画のスプリントと同じ。
type ReportSprint struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	State     string           `json:"state" enum:"planned,active,closed"`
	Start     string           `json:"start" doc:"YYYY-MM-DD。空は値なし"`
	End       string           `json:"end"`
	Goal      string           `json:"goal"`
	Confirmed *ReportConfirmed `json:"confirmed" doc:"完了で確定した値。完了していなければ null"`
}

// ReportSprintOrNull は、スプリント。なければ null。
type ReportSprintOrNull struct {
	_ struct{} `nullable:"true"`
	ReportSprint
}

// ReportBurnDay は、バーンダウンの 1 日。remaining は表示する日より後で null。
type ReportBurnDay struct {
	Date      string   `json:"date"`
	Total     float64  `json:"total"`
	Remaining *float64 `json:"remaining"`
	Ideal     float64  `json:"ideal" doc:"理想の線の値。小数第 2 位を四捨五入した小数第 1 位まで"`
}

// ReportBurndown は、スプリントのバーンダウン。
type ReportBurndown struct {
	Days      []ReportBurnDay `json:"days" nullable:"false"`
	Shown     int             `json:"shown" doc:"表示する日の数"`
	Committed *float64        `json:"committed" doc:"完了したスプリントで総量が確定の計画と違うときだけ、その確定の計画"`
}

// ReportVelocity は、ベロシティの 1 本。
type ReportVelocity struct {
	Sprint  ReportSprint `json:"sprint"`
	Planned float64      `json:"planned"`
	Done    float64      `json:"done"`
}

// ReportPeriod は、累積フロー図の期間。
type ReportPeriod struct {
	From string `json:"from" doc:"YYYY-MM-DD"`
	To   string `json:"to" doc:"YYYY-MM-DD"`
}

// ReportLayer は、累積フロー図の層。分類の層の color は空。
type ReportLayer struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// ReportFlowDay は、累積フロー図の 1 日。counts は layers と同じ順。
type ReportFlowDay struct {
	Date   string `json:"date"`
	Counts []int  `json:"counts" nullable:"false"`
}

// ReportDurations は、サイクルタイムまたはリードタイムの集計(日数)。
type ReportDurations struct {
	N       int      `json:"n"`
	Median  *float64 `json:"median"`
	P85     *float64 `json:"p85"`
	Skipped int      `json:"skipped" doc:"数えなかった件数"`
}

// ReportEstimate は、見積もりと実績の表の 1 行。時間は 100 分の 1 時間の整数。
type ReportEstimate struct {
	ID       string   `json:"id"`
	Display  string   `json:"display"`
	Title    string   `json:"title"`
	Points   *float64 `json:"points"`
	Estimate *int     `json:"estimate"`
	Worked   int      `json:"worked"`
	Diff     *int     `json:"diff"`
	PerPoint *float64 `json:"perPoint"`
}

// ReportEstimateTotal は、表の合計の行。時間は 100 分の 1 時間の整数。
type ReportEstimateTotal struct {
	Points   *float64 `json:"points"`
	Estimate *int     `json:"estimate"`
	Worked   int      `json:"worked"`
	Diff     *int     `json:"diff"`
}

// ReportView は、レポートの全部の数字。
type ReportView struct {
	Selection     ReportSelection     `json:"selection"`
	Unit          string              `json:"unit" enum:"points,count" doc:"実際に数えた単位"`
	Fallback      bool                `json:"fallback" doc:"ポイントを選んでいるが件数で数えたか"`
	Sprints       []ReportSprint      `json:"sprints" nullable:"false" doc:"バーンダウンと見積もりと実績の選択肢"`
	Burn          *ReportSprintOrNull `json:"burn" doc:"バーンダウンのスプリント。未選択なら null"`
	Burndown      ReportBurndown      `json:"burndown"`
	Velocity      []ReportVelocity    `json:"velocity" nullable:"false"`
	Average       *float64            `json:"average" doc:"完了した直近 3 スプリントの平均。なければ null"`
	Period        ReportPeriod        `json:"period"`
	Layers        []ReportLayer       `json:"layers" nullable:"false"`
	Flow          []ReportFlowDay     `json:"flow" nullable:"false"`
	Cycle         ReportDurations     `json:"cycle"`
	Lead          ReportDurations     `json:"lead"`
	Estimates     []ReportEstimate    `json:"estimates" nullable:"false"`
	EstimateTotal ReportEstimateTotal `json:"estimateTotal"`
}

// ReportActive は、実行中のスプリントの数字。sprint が null のときほかは空。
type ReportActive struct {
	Sprint   *ReportSprintOrNull `json:"sprint" doc:"実行中のスプリント。なければ null"`
	Unit     string              `json:"unit" enum:"points,count"`
	Burndown ReportBurndown      `json:"burndown"`
	Added    float64             `json:"added" doc:"途中で追加した量"`
	Done     ReportTotals        `json:"done"`
	Total    ReportTotals        `json:"total"`
}

func totals(t planning.Totals) ReportTotals { return ReportTotals{Points: t.Points, Count: t.Count} }

func toSprint(s planning.Sprint) ReportSprint {
	out := ReportSprint{ID: string(s.ID), Name: s.Name, State: string(s.State), Start: s.Start, End: s.End, Goal: s.Goal}
	if s.Confirmed != nil {
		out.Confirmed = &ReportConfirmed{Committed: totals(s.Confirmed.Committed), Completed: totals(s.Confirmed.Completed)}
	}
	return out
}

func toSprintOrNull(s *planning.Sprint) *ReportSprintOrNull {
	if s == nil {
		return nil
	}
	return &ReportSprintOrNull{ReportSprint: toSprint(*s)}
}

func toBurndown(b planning.Burndown) ReportBurndown {
	out := ReportBurndown{Days: []ReportBurnDay{}, Shown: b.Shown, Committed: b.Committed}
	for _, d := range b.Days {
		out.Days = append(out.Days, ReportBurnDay{Date: d.Date, Total: d.Total, Remaining: d.Remaining, Ideal: d.Ideal})
	}
	return out
}

func toDurations(d planning.Durations) ReportDurations {
	return ReportDurations{N: d.N, Median: d.Median, P85: d.P85, Skipped: d.Skipped}
}

func toSelection(s planning.Selection) ReportSelection {
	return ReportSelection{Unit: string(s.Unit), Sprint: string(s.Sprint), Layers: string(s.Layers), Estimate: string(s.Estimate)}
}

func toView(r appreports.Report) ReportView {
	out := ReportView{
		Selection: toSelection(r.Selection), Unit: string(r.Unit), Fallback: r.Fallback,
		Sprints: []ReportSprint{}, Burn: toSprintOrNull(r.Burn), Burndown: toBurndown(r.Burndown),
		Velocity: []ReportVelocity{}, Average: r.Average, Period: ReportPeriod{From: r.Period.From, To: r.Period.To},
		Layers: []ReportLayer{}, Flow: []ReportFlowDay{}, Cycle: toDurations(r.Cycle), Lead: toDurations(r.Lead),
		Estimates: []ReportEstimate{},
		EstimateTotal: ReportEstimateTotal{
			Points: r.EstimateTotal.Points, Estimate: r.EstimateTotal.Estimate,
			Worked: r.EstimateTotal.Worked, Diff: r.EstimateTotal.Diff,
		},
	}
	for _, s := range r.Sprints {
		out.Sprints = append(out.Sprints, toSprint(s))
	}
	for _, v := range r.Velocity {
		out.Velocity = append(out.Velocity, ReportVelocity{Sprint: toSprint(v.Sprint), Planned: v.Planned, Done: v.Done})
	}
	for _, l := range r.Layers {
		out.Layers = append(out.Layers, ReportLayer{Key: l.Key, Name: l.Name, Color: l.Color})
	}
	for _, d := range r.Flow {
		counts := d.Counts
		if counts == nil {
			counts = []int{}
		}
		out.Flow = append(out.Flow, ReportFlowDay{Date: d.Date, Counts: counts})
	}
	for _, e := range r.Estimates {
		out.Estimates = append(out.Estimates, ReportEstimate{
			ID: string(e.Item.ID), Display: e.Display, Title: e.Item.Title,
			Points: e.Points, Estimate: e.Estimate, Worked: e.Worked, Diff: e.Diff, PerPoint: e.PerPoint,
		})
	}
	return out
}

func toActive(a appreports.Active) ReportActive {
	return ReportActive{
		Sprint: toSprintOrNull(a.Sprint), Unit: string(a.Unit), Burndown: toBurndown(a.Burndown),
		Added: a.Added, Done: totals(a.Done), Total: totals(a.Total),
	}
}
