package reports_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/reports"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	dp "github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/worklog"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const store port.StoreID = "S1"

const (
	sprint1 change.ID = "01SAMPLESP0000000000000001"
	sprint2 change.ID = "01SAMPLESP0000000000000002"
	sprint3 change.ID = "01SAMPLESP0000000000000003"
)

const (
	itemA item.ID = "01WORKITEM00000000000000A1"
	itemB item.ID = "01WORKITEM00000000000000A2"
	itemC item.ID = "01WORKITEM00000000000000A3"
)

const (
	doing  = apptest.SampleStatusInProgress
	done   = apptest.SampleStatusDone
	cancel = apptest.SampleStatusCancel
)

// fakeSelections は、メモリ上の Selections。puts は Put の回数。
type fakeSelections struct {
	rows map[workspace.ID]dp.Selection
	puts int
}

func (f *fakeSelections) Get(_ context.Context, _ port.StoreID, ws workspace.ID) (dp.Selection, bool, error) {
	sel, ok := f.rows[ws]
	return sel, ok, nil
}

func (f *fakeSelections) Put(_ context.Context, _ port.StoreID, ws workspace.ID, sel dp.Selection) error {
	if f.rows == nil {
		f.rows = map[workspace.ID]dp.Selection{}
	}
	f.rows[ws] = sel
	f.puts++
	return nil
}

// readOnly は、Write を断る Store(閲覧のみの Store)。
type readOnly struct{ port.Stores }

func (readOnly) Write(context.Context, port.StoreID, port.Operation, func(port.Tx) error) error {
	return errors.New("read only")
}

func setup(t *testing.T) (reports.Service, *fakeSelections, port.Stores) {
	t.Helper()
	ids := apptest.NewIDs()
	clock := apptest.NewClock(time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC))
	s := apptest.NewStores(clock, ids)
	s.AddStore(store)
	if err := apptest.Sample(context.Background(), s, store); err != nil {
		t.Fatal(err)
	}
	sel := &fakeSelections{}
	return reports.Service{Stores: s, Selections: sel, Clock: clock}, sel, s
}

func writeWS(t *testing.T, s port.Stores, ds ...change.Draft) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "計画を変更"}
	err := s.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, ds...)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// addSprint は、スプリントを足す。state が計画中でなければ、その状態にする。
func addSprint(t *testing.T, s port.Stores, id change.ID, name string, state dp.SprintState, start, end string) {
	t.Helper()
	sp := dp.Sprint{ID: id, Name: name, State: dp.Planned}
	writeWS(t, s, dp.SprintDrafts(dp.Sprint{}, sp)...)
	if state != dp.Planned {
		next := sp
		next.State, next.Start, next.End = state, start, end
		writeWS(t, s, dp.SprintDrafts(sp, next)...)
	}
}

type work struct {
	id     item.ID
	number int
	status change.ID
	sprint change.ID
	points float64 // 負なら未設定
}

func addWorks(t *testing.T, s port.Stores, ws ...work) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "アイテムを作成"}
	err := s.Write(t.Context(), store, op, func(tx port.Tx) error {
		for _, w := range ws {
			it := item.Item{
				ID:    w.id,
				Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Number: w.number},
				Kind:  item.Task, Rank: string(w.id), Title: string(w.id), Status: w.status, Sprint: w.sprint,
			}
			if w.points >= 0 {
				p := w.points
				it.Points = &p
			}
			if err := tx.WriteWorkspaces().Append(apptest.SampleWorkspace, numbering.Draft(numbering.Items, w.number)); err != nil {
				return err
			}
			if err := tx.WriteItems().Append(w.id, item.Create(it)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// activeWithWork は、実行中の Sprint 1(11-10〜11-20)と、ポイントのある作業項目 3 件(完了 3・進行中 2・中止 1)を足す。
func activeWithWork(t *testing.T, s port.Stores) {
	t.Helper()
	addSprint(t, s, sprint1, "Sprint 1", dp.Active, "2026-11-10", "2026-11-20")
	addWorks(t, s,
		work{itemA, 3, done, sprint1, 3},
		work{itemB, 4, doing, sprint1, 2},
		work{itemC, 5, cancel, sprint1, 1},
	)
}

func TestReportDefaults(t *testing.T) {
	svc, _, s := setup(t)
	activeWithWork(t, s)
	r, err := svc.Report(t.Context(), store, apptest.SampleWorkspace, reports.Period{})
	if err != nil {
		t.Fatal(err)
	}
	want := dp.Selection{Unit: dp.UnitPoints, Sprint: sprint1, Layers: dp.LayersCategory}
	if r.Selection != want || r.Unit != dp.UnitPoints || r.Fallback {
		t.Errorf("Selection = %+v, Unit = %s, Fallback = %v", r.Selection, r.Unit, r.Fallback)
	}
	if r.Burn == nil || r.Burn.ID != sprint1 || r.Burndown.Shown != 5 {
		t.Errorf("Burn = %+v, Shown = %d", r.Burn, r.Burndown.Shown)
	}
	if r.Period != (reports.Period{From: "2026-10-25", To: "2026-11-14"}) {
		t.Errorf("Period = %+v", r.Period)
	}
	if len(r.Sprints) != 1 || len(r.Flow) != 21 || len(r.Layers) == 0 || len(r.Velocity) == 0 {
		t.Errorf("Sprints = %d, Flow = %d, Layers = %d, Velocity = %d", len(r.Sprints), len(r.Flow), len(r.Layers), len(r.Velocity))
	}
	if len(r.Estimates) != 0 {
		t.Errorf("Estimates = %+v", r.Estimates)
	}
}

func TestReportSavedSprintGone(t *testing.T) {
	t.Run("バーンダウンのスプリントを削除すると実行中を選ぶ", func(t *testing.T) {
		svc, sel, s := setup(t)
		activeWithWork(t, s)
		addSprint(t, s, sprint2, "Sprint 2", dp.Closed, "2026-11-01", "2026-11-09")
		sel.rows = map[workspace.ID]dp.Selection{apptest.SampleWorkspace: {Sprint: sprint2}}
		writeWS(t, s, dp.DeleteSprint(sprint2))
		r, err := svc.Report(t.Context(), store, apptest.SampleWorkspace, reports.Period{})
		if err != nil {
			t.Fatal(err)
		}
		if r.Selection.Sprint != sprint1 {
			t.Errorf("Sprint = %q", r.Selection.Sprint)
		}
	})
	t.Run("対象のスプリントを削除するとすべてになり、バーンダウンの選択は残る", func(t *testing.T) {
		svc, sel, s := setup(t)
		activeWithWork(t, s)
		addSprint(t, s, sprint2, "Sprint 2", dp.Closed, "2026-11-01", "2026-11-09")
		addSprint(t, s, sprint3, "Sprint 3", dp.Closed, "2026-10-01", "2026-10-09")
		sel.rows = map[workspace.ID]dp.Selection{apptest.SampleWorkspace: {Sprint: sprint2, Estimate: sprint3}}
		writeWS(t, s, dp.DeleteSprint(sprint3))
		r, err := svc.Report(t.Context(), store, apptest.SampleWorkspace, reports.Period{})
		if err != nil {
			t.Fatal(err)
		}
		if r.Selection.Sprint != sprint2 || r.Selection.Estimate != "" {
			t.Errorf("Selection = %+v", r.Selection)
		}
	})
}

func TestReportFallback(t *testing.T) {
	svc, _, s := setup(t)
	addSprint(t, s, sprint1, "Sprint 1", dp.Active, "2026-11-10", "2026-11-20")
	r, err := svc.Report(t.Context(), store, apptest.SampleWorkspace, reports.Period{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Unit != dp.UnitCount || !r.Fallback || r.Selection.Unit != dp.UnitPoints {
		t.Errorf("Unit = %s, Fallback = %v, Selection.Unit = %s", r.Unit, r.Fallback, r.Selection.Unit)
	}
	a, err := svc.Active(t.Context(), store, apptest.SampleWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	if a.Unit != dp.UnitCount {
		t.Errorf("Active.Unit = %s", a.Unit)
	}
}

func TestReportPeriod(t *testing.T) {
	svc, _, _ := setup(t)
	_, err := svc.Report(t.Context(), store, apptest.SampleWorkspace, reports.Period{From: "2026-11-01", To: "2026-11-15"})
	var rf *refusal.Refusal
	if !errors.As(err, &rf) || rf.Message != "終了日は今日以前にしてください" {
		t.Errorf("err = %v", err)
	}
	r, err := svc.Report(t.Context(), store, apptest.SampleWorkspace, reports.Period{From: "2026-11-10", To: "2026-11-12"})
	if err != nil || len(r.Flow) != 3 || r.Period.From != "2026-11-10" {
		t.Errorf("Flow = %d, Period = %+v, err = %v", len(r.Flow), r.Period, err)
	}
}

func TestReportEstimatesDisplay(t *testing.T) {
	svc, _, s := setup(t)
	activeWithWork(t, s)
	op := port.Operation{Actor: change.User("taro"), Summary: "作業時間を記録"}
	err := s.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteItems().Append(itemA, change.NewWorklog("01WORKLOG00000000000000001", nil, worklog.Value(150, "", "2026-11-12")))
	})
	if err != nil {
		t.Fatal(err)
	}
	r, err := svc.Report(t.Context(), store, apptest.SampleWorkspace, reports.Period{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Estimates) != 1 || r.Estimates[0].Display != "HH-3" || r.Estimates[0].Worked != 150 || r.EstimateTotal.Worked != 150 {
		t.Errorf("Estimates = %+v, Total = %+v", r.Estimates, r.EstimateTotal)
	}
}

func TestReportGoneWorkspace(t *testing.T) {
	svc, _, s := setup(t)
	if err := apptest.DeleteWorkspace(t.Context(), s, store, apptest.SampleWorkspace); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Report(t.Context(), store, apptest.SampleWorkspace, reports.Period{}); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Report err = %v", err)
	}
	if _, err := svc.Active(t.Context(), store, apptest.SampleWorkspace); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Active err = %v", err)
	}
}
