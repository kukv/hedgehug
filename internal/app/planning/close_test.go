package planning_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/planning"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/history"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	dp "github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/rank"
)

// work は、テストで足す作業項目。
type work struct {
	id     item.ID
	number int
	rank   string
	status change.ID
	sprint change.ID
	points float64
}

// addWorks は、Task を 1 回の保存で足す。
func addWorks(t *testing.T, svc planning.Service, ws []work) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "アイテムを作成"}
	err := svc.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		for _, w := range ws {
			pt := w.points
			it := item.Item{
				ID:    w.id,
				Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Number: w.number},
				Kind:  item.Task, Rank: w.rank, Title: string(w.id), Status: w.status, Sprint: w.sprint, Points: &pt,
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

func getItem(t *testing.T, svc planning.Service, id item.ID) (item.Item, []change.Record) {
	t.Helper()
	var it item.Item
	var log []change.Record
	err := svc.Stores.Read(t.Context(), store, func(r port.Reader) error {
		var err error
		if it, err = r.Items().Get(id); err != nil {
			return err
		}
		log, err = r.Changes().Of(change.Ref{Kind: change.KindItem, ID: id})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return it, log
}

func sprintChanges(log []change.Record) int {
	n := 0
	for _, r := range history.ItemRows(log, history.Context{Names: history.NamesOf(nil)}) {
		if r.Text == "スプリントを変更" {
			n++
		}
	}
	return n
}

func sprintOf(t *testing.T, svc planning.Service, id change.ID) dp.Sprint {
	t.Helper()
	for _, sp := range sprints(t, svc) {
		if sp.ID == id {
			return sp
		}
	}
	t.Fatalf("sprint %s がない", id)
	return dp.Sprint{}
}

const (
	itemA item.ID = "01CLOSEIT0000000000000000A"
	itemB item.ID = "01CLOSEIT0000000000000000B"
	itemC item.ID = "01CLOSEIT0000000000000000C"
	itemD item.ID = "01CLOSEIT0000000000000000D"
	itemE item.ID = "01CLOSEIT0000000000000000E"
	itemX item.ID = "01CLOSEIT0000000000000000X"
)

// activeSprint は、実行中の Sprint 1 と計画中の Sprint 2 を作る。
func activeSprint(t *testing.T, svc planning.Service) (s1, s2 dp.Sprint) {
	t.Helper()
	ctx := t.Context()
	s1, err := svc.CreateSprint(ctx, store, apptest.SampleWorkspace, planning.SprintInput{Name: "Sprint 1"})
	if err != nil {
		t.Fatal(err)
	}
	s2, err = svc.CreateSprint(ctx, store, apptest.SampleWorkspace, planning.SprintInput{Name: "Sprint 2"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.StartSprint(ctx, store, apptest.SampleWorkspace, s1.ID, "2026-11-13", "2026-11-27"); err != nil {
		t.Fatal(err)
	}
	return s1, s2
}

// sprintWorks は、S1 に並び順 A・B・C・D・E の作業項目を足す。番号の順(C が先)は並び順と違える。
// E は削除済みにする。
func sprintWorks(t *testing.T, svc planning.Service, s1 change.ID) {
	t.Helper()
	addWorks(t, svc, []work{
		{itemC, 3, "D", apptest.SampleStatusTodo, s1, 2},
		{itemA, 4, "B", apptest.SampleStatusInProgress, s1, 3},
		{itemB, 5, "C", apptest.SampleStatusDone, s1, 5},
		{itemD, 6, "E", apptest.SampleStatusCancel, s1, 8},
		{itemE, 7, "F", apptest.SampleStatusTodo, s1, 1},
	})
	if err := apptest.DeleteItems(t.Context(), svc.Stores, store, itemE); err != nil {
		t.Fatal(err)
	}
}

func TestCloseSprintMovesUndone(t *testing.T) {
	svc, rec := setup(t)
	s1, s2 := activeSprint(t, svc)
	sprintWorks(t, svc, s1.ID)
	addWorks(t, svc, []work{{itemX, 8, "X", apptest.SampleStatusTodo, s2.ID, 1}})
	_, eLog := getItem(t, svc, itemE)

	dest := s2.ID
	if err := svc.CloseSprint(t.Context(), store, apptest.SampleWorkspace, s1.ID, &dest); err != nil {
		t.Fatal(err)
	}
	if got := lastSummary(rec); got != "スプリント「Sprint 1」を完了" {
		t.Errorf("summary = %q", got)
	}
	got := sprintOf(t, svc, s1.ID)
	want := dp.Confirmed{Committed: dp.Totals{Points: 10, Count: 3}, Completed: dp.Totals{Points: 5, Count: 1}}
	if got.State != dp.Closed || got.Confirmed == nil || *got.Confirmed != want {
		t.Errorf("S1 = %+v %+v", got, got.Confirmed)
	}
	a, aLog := getItem(t, svc, itemA)
	c, cLog := getItem(t, svc, itemC)
	if a.Sprint != s2.ID || c.Sprint != s2.ID {
		t.Errorf("移動先: A=%s C=%s", a.Sprint, c.Sprint)
	}
	if a.Rank != rank.After("X") || c.Rank != rank.After(rank.After("X")) {
		t.Errorf("rank: A=%s C=%s", a.Rank, c.Rank)
	}
	if sprintChanges(aLog) != 1 || sprintChanges(cLog) != 1 {
		t.Errorf("履歴: A=%d C=%d", sprintChanges(aLog), sprintChanges(cLog))
	}
	for _, id := range []item.ID{itemB, itemD} {
		if it, _ := getItem(t, svc, id); it.Sprint != s1.ID {
			t.Errorf("%s が S1 にない: %s", id, it.Sprint)
		}
	}
	if e, log := getItem(t, svc, itemE); e.Sprint != s1.ID || len(log) != len(eLog) {
		t.Errorf("E に触れた: %s %d→%d", e.Sprint, len(eLog), len(log))
	}
}

func TestCloseSprintToEmptyBacklog(t *testing.T) {
	svc, _ := setup(t)
	s1, _ := activeSprint(t, svc)
	sprintWorks(t, svc, s1.ID)
	// サンプルの Task はバックログの枠にあるので、削除して枠を空にする。
	if err := apptest.DeleteItems(t.Context(), svc.Stores, store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseSprint(t.Context(), store, apptest.SampleWorkspace, s1.ID, nil); err != nil {
		t.Fatal(err)
	}
	a, aLog := getItem(t, svc, itemA)
	c, _ := getItem(t, svc, itemC)
	if a.Sprint != "" || c.Sprint != "" || a.Rank != "B" || c.Rank != "D" {
		t.Errorf("A=%s/%s C=%s/%s", a.Sprint, a.Rank, c.Sprint, c.Rank)
	}
	for _, r := range aLog {
		if r.F == "rank" {
			t.Errorf("rank の行がある: %+v", r)
		}
	}
	if sprintOf(t, svc, s1.ID).State != dp.Closed {
		t.Error("S1 が完了でない")
	}
}

// バックログの枠にアイテムがあれば、その末尾に順に続ける。
func TestCloseSprintToBacklogEnd(t *testing.T) {
	svc, _ := setup(t)
	s1, _ := activeSprint(t, svc)
	sprintWorks(t, svc, s1.ID)
	addWorks(t, svc, []work{{itemX, 8, "X", apptest.SampleStatusTodo, "", 1}})
	if err := svc.CloseSprint(t.Context(), store, apptest.SampleWorkspace, s1.ID, nil); err != nil {
		t.Fatal(err)
	}
	a, _ := getItem(t, svc, itemA)
	c, _ := getItem(t, svc, itemC)
	if a.Sprint != "" || c.Sprint != "" || a.Rank != rank.After("X") || c.Rank != rank.After(rank.After("X")) {
		t.Errorf("A=%s/%s C=%s/%s", a.Sprint, a.Rank, c.Sprint, c.Rank)
	}
}

func TestCloseSprintRefusals(t *testing.T) {
	svc, _ := setup(t)
	ctx := t.Context()
	s1, s2 := activeSprint(t, svc)
	s3, err := svc.CreateSprint(ctx, store, apptest.SampleWorkspace, planning.SprintInput{Name: "Sprint 3"})
	if err != nil {
		t.Fatal(err)
	}
	op := port.Operation{Actor: change.User("taro"), Summary: "スプリント「Sprint 3」を削除"}
	err = svc.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, dp.DeleteSprint(s3.ID))
	})
	if err != nil {
		t.Fatal(err)
	}
	unknown := change.ID("01NOSUCHSPRINT0000000000")
	active, deleted := s1.ID, s3.ID
	for _, c := range []struct {
		name string
		id   change.ID
		dest *change.ID
	}{
		{"計画中のスプリント", s2.ID, nil},
		{"ないスプリント", unknown, nil},
		{"移動先が実行中", s1.ID, &active},
		{"移動先が削除済み", s1.ID, &deleted},
		{"移動先がない", s1.ID, &unknown},
	} {
		if err := svc.CloseSprint(ctx, store, apptest.SampleWorkspace, c.id, c.dest); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	if err := svc.CloseSprint(ctx, store, apptest.SampleWorkspace, s1.ID, nil); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseSprint(ctx, store, apptest.SampleWorkspace, s1.ID, nil); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("完了済みのスプリント: %v", err)
	}
	if err := svc.StartSprint(ctx, store, apptest.SampleWorkspace, s2.ID, "2026-11-13", "2026-11-27"); err != nil {
		t.Fatal(err)
	}
	closed := s1.ID
	if err := svc.CloseSprint(ctx, store, apptest.SampleWorkspace, s2.ID, &closed); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("移動先が完了: %v", err)
	}
}

func TestCloseSprintZero(t *testing.T) {
	svc, rec := setup(t)
	s1, _ := activeSprint(t, svc)
	if err := svc.CloseSprint(t.Context(), store, apptest.SampleWorkspace, s1.ID, nil); err != nil {
		t.Fatal(err)
	}
	got := sprintOf(t, svc, s1.ID)
	if got.State != dp.Closed || got.Confirmed == nil || *got.Confirmed != (dp.Confirmed{}) {
		t.Errorf("S1 = %+v %+v", got, got.Confirmed)
	}
	if lastSummary(rec) != "スプリント「Sprint 1」を完了" {
		t.Errorf("summary = %q", lastSummary(rec))
	}
}
