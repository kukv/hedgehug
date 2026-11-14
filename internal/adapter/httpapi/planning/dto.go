package planning

import (
	appplanning "github.com/kukv/hedgehug/internal/app/planning"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/planning"
)

// SprintInput は、スプリントの作成と編集の本文。
type SprintInput struct {
	Name  string `json:"name"`
	Start string `json:"start" pattern:"^(\\d{4}-\\d{2}-\\d{2})?$" doc:"YYYY-MM-DD。空は値なし"`
	End   string `json:"end" pattern:"^(\\d{4}-\\d{2}-\\d{2})?$" doc:"YYYY-MM-DD。空は値なし"`
	Goal  string `json:"goal"`
}

// SprintTotals は、ポイントと件数の合計。
type SprintTotals struct {
	Points float64 `json:"points"`
	Count  int     `json:"count"`
}

// SprintConfirmed は、完了で確定した計画(committed)と実績(completed)。完了していなければ null。
type SprintConfirmed struct {
	_         struct{}     `nullable:"true"`
	Committed SprintTotals `json:"committed"`
	Completed SprintTotals `json:"completed"`
}

// Sprint は、スプリント。
type Sprint struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	State     string           `json:"state" enum:"planned,active,closed"`
	Start     string           `json:"start" doc:"YYYY-MM-DD。空は値なし"`
	End       string           `json:"end"`
	Goal      string           `json:"goal"`
	Confirmed *SprintConfirmed `json:"confirmed" doc:"完了で確定した値。完了していなければ null"`
}

// ReleaseInput は、リリースの作成と編集の本文。
type ReleaseInput struct {
	Name        string `json:"name"`
	Date        string `json:"date" pattern:"^(\\d{4}-\\d{2}-\\d{2})?$" doc:"YYYY-MM-DD。空は値なし"`
	Description string `json:"description"`
}

// Release は、リリース。
type Release struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Date        string `json:"date"`
	Description string `json:"description"`
	State       string `json:"state" enum:"unreleased,released,archived"`
}

// ActiveSprint は、実行中のスプリント。なければ null。
type ActiveSprint struct {
	_ struct{} `nullable:"true"`
	Sprint
}

// SprintDefaults は、スプリントの作成の既定。
type SprintDefaults struct {
	Name  string `json:"name"`
	Start string `json:"start"`
	End   string `json:"end"`
}

// PlanningOverview は、スプリントとリリースの選択肢、実行中のスプリント、作成の既定。削除済みを除く。
type PlanningOverview struct {
	Sprints  []Sprint       `json:"sprints" nullable:"false"`
	Releases []Release      `json:"releases" nullable:"false"`
	Active   *ActiveSprint  `json:"active"`
	Defaults SprintDefaults `json:"defaults"`
	// RetentionDays は、スプリントの削除の確認の文に使う(05「削除」)。
	RetentionDays int `json:"retentionDays" doc:"その Store の削除済みの保持日数"`
}

// PlanningRow は、カードのアイテムの 1 行。
type PlanningRow struct {
	ID       string   `json:"id"`
	Display  string   `json:"display"`
	Kind     string   `json:"kind" enum:"Epic,Story,Task,Bug,Step"`
	Title    string   `json:"title"`
	Points   *float64 `json:"points"`
	Status   string   `json:"status" doc:"ステータスの内部 ID。空は値なし"`
	Assignee string   `json:"assignee" doc:"ログイン名。空は未割り当て"`
}

// SprintCard は、スプリントのカード。total・done は中止を除く。
type SprintCard struct {
	Sprint   Sprint        `json:"sprint"`
	Total    SprintTotals  `json:"total"`
	Done     SprintTotals  `json:"done"`
	Undone   int           `json:"undone" doc:"未着手・進行中の件数"`
	Canceled int           `json:"canceled"`
	Items    []PlanningRow `json:"items" nullable:"false"`
}

// ReleaseCard は、リリースのカード。target は Story・Task・Bug の中止でないもの。
type ReleaseCard struct {
	Release Release       `json:"release"`
	Target  int           `json:"target"`
	Done    int           `json:"done"`
	Undone  int           `json:"undone"`
	Items   []PlanningRow `json:"items" nullable:"false"`
}

// StartSprint は、スプリントの開始の本文。
type StartSprint struct {
	Start string `json:"start" pattern:"^(\\d{4}-\\d{2}-\\d{2})?$" doc:"YYYY-MM-DD"`
	End   string `json:"end" pattern:"^(\\d{4}-\\d{2}-\\d{2})?$" doc:"YYYY-MM-DD"`
}

// CloseSprint は、スプリントの完了の本文。
type CloseSprint struct {
	Destination *string `json:"destination" doc:"未完了のアイテムの移動先(計画中のスプリントの内部 ID)。null はバックログ"`
}

// SetReleaseState は、リリースの状態の変更の本文。
type SetReleaseState struct {
	State string `json:"state" enum:"unreleased,released,archived"`
}

func totals(t planning.Totals) SprintTotals { return SprintTotals{Points: t.Points, Count: t.Count} }

func toSprint(s planning.Sprint) Sprint {
	out := Sprint{ID: string(s.ID), Name: s.Name, State: string(s.State), Start: s.Start, End: s.End, Goal: s.Goal}
	if s.Confirmed != nil {
		out.Confirmed = &SprintConfirmed{Committed: totals(s.Confirmed.Committed), Completed: totals(s.Confirmed.Completed)}
	}
	return out
}

func toRelease(r planning.Release) Release {
	return Release{ID: string(r.ID), Name: r.Name, Date: r.Date, Description: r.Description, State: string(r.State)}
}

func toOverview(v appplanning.Overview) PlanningOverview {
	out := PlanningOverview{
		Sprints: []Sprint{}, Releases: []Release{}, RetentionDays: v.RetentionDays,
		Defaults: SprintDefaults{Name: v.Defaults.Name, Start: v.Defaults.Start, End: v.Defaults.End},
	}
	for _, s := range v.Sprints {
		out.Sprints = append(out.Sprints, toSprint(s))
	}
	for _, r := range v.Releases {
		out.Releases = append(out.Releases, toRelease(r))
	}
	if v.Active != nil {
		out.Active = &ActiveSprint{Sprint: toSprint(*v.Active)}
	}
	return out
}

func toRows(rows []appplanning.Row) []PlanningRow {
	out := []PlanningRow{}
	for _, r := range rows {
		out = append(out, PlanningRow{
			ID: string(r.ID), Display: r.Display, Kind: string(r.Kind), Title: r.Title,
			Points: r.Points, Status: string(r.Status), Assignee: r.Assignee,
		})
	}
	return out
}

func toSprintCards(cs []appplanning.SprintCard) []SprintCard {
	out := []SprintCard{}
	for _, c := range cs {
		out = append(out, SprintCard{
			Sprint: toSprint(c.Sprint), Total: totals(c.Total), Done: totals(c.Done),
			Undone: c.Undone, Canceled: c.Canceled, Items: toRows(c.Items),
		})
	}
	return out
}

func toReleaseCards(cs []appplanning.ReleaseCard) []ReleaseCard {
	out := []ReleaseCard{}
	for _, c := range cs {
		out = append(out, ReleaseCard{Release: toRelease(c.Release), Target: c.Target, Done: c.Done, Undone: c.Undone, Items: toRows(c.Items)})
	}
	return out
}

// ref は、本文の内部 ID。null と空は nil(バックログ・なし)。
func ref(s *string) *change.ID {
	if s == nil || *s == "" {
		return nil
	}
	id := change.ID(*s)
	return &id
}
