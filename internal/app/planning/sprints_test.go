package planning_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/planning"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	dp "github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

const store port.StoreID = "S1"

// recorder は、保存のアクティビティの文を覚える。
type recorder struct {
	port.Stores
	summaries []string
}

func (r *recorder) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(port.Tx) error) error {
	r.summaries = append(r.summaries, op.Summary)
	return r.Stores.Write(ctx, id, op, fn)
}

func setup(t *testing.T) (planning.Service, *recorder) {
	t.Helper()
	ids := apptest.NewIDs()
	clock := apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))
	s := apptest.NewStores(clock, ids)
	s.AddStore(store)
	if err := apptest.Sample(context.Background(), s, store); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{Stores: s}
	return planning.Service{Stores: rec, IDs: ids, Identity: apptest.Identity{Login: "hanako"}, Clock: clock}, rec
}

func message(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}

// sprints は、保存されているスプリント(削除済みを除く)。
func sprints(t *testing.T, svc planning.Service) []dp.Sprint {
	t.Helper()
	var out []dp.Sprint
	err := svc.Stores.Read(t.Context(), store, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace})
		out = dp.FromLog(log).LiveSprints()
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func lastSummary(rec *recorder) string { return rec.summaries[len(rec.summaries)-1] }

func TestCreateSprint(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	sp, err := svc.CreateSprint(ctx, store, apptest.SampleWorkspace, planning.SprintInput{Name: " Sprint 1 ", Goal: "g"})
	if err != nil {
		t.Fatal(err)
	}
	if sp.ID == "" || sp.Name != "Sprint 1" || sp.State != dp.Planned || sp.Goal != "g" {
		t.Errorf("CreateSprint = %+v", sp)
	}
	if got := lastSummary(rec); got != "スプリント「Sprint 1」を作成" {
		t.Errorf("summary = %q", got)
	}
	if got := sprints(t, svc); len(got) != 1 || got[0].ID != sp.ID || got[0].State != dp.Planned {
		t.Errorf("stored = %+v", got)
	}
	_, err = svc.CreateSprint(ctx, store, apptest.SampleWorkspace, planning.SprintInput{Name: "Sprint 1"})
	if message(err) != "同じ名前のスプリントがあります" {
		t.Errorf("duplicate: %v", err)
	}
	if got := sprints(t, svc); len(got) != 1 {
		t.Errorf("duplicate saved: %+v", got)
	}
}

func TestUpdateSprint(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	sp, _ := svc.CreateSprint(ctx, store, apptest.SampleWorkspace, planning.SprintInput{Name: "Sprint 1"})
	in := planning.SprintInput{Name: " Sprint 1b ", Start: "2026-11-02", End: "2026-11-15", Goal: "新しい目標"}
	if err := svc.UpdateSprint(ctx, store, apptest.SampleWorkspace, sp.ID, in); err != nil {
		t.Fatal(err)
	}
	if got := lastSummary(rec); got != "スプリント「Sprint 1b」を編集" {
		t.Errorf("summary = %q", got)
	}
	got := sprints(t, svc)[0]
	if got.Name != "Sprint 1b" || got.Start != "2026-11-02" || got.End != "2026-11-15" || got.Goal != "新しい目標" || got.State != dp.Planned {
		t.Errorf("stored = %+v", got)
	}
	if err := svc.StartSprint(ctx, store, apptest.SampleWorkspace, sp.ID, "2026-11-02", "2026-11-15"); err != nil {
		t.Fatal(err)
	}
	in.End = ""
	err := svc.UpdateSprint(ctx, store, apptest.SampleWorkspace, sp.ID, in)
	if message(err) != "実行中のスプリントには開始日と終了日が必要です" {
		t.Errorf("empty end: %v", err)
	}
	if got := sprints(t, svc)[0]; got.State != dp.Active || got.End != "2026-11-15" {
		t.Errorf("stored after refusal = %+v", got)
	}
	err = svc.UpdateSprint(ctx, store, apptest.SampleWorkspace, "01NOSUCHSPRINT0000000000", in)
	if !errors.Is(err, port.ErrNotFound) {
		t.Errorf("unknown: %v", err)
	}
}

func TestStartSprint(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	a, _ := svc.CreateSprint(ctx, store, apptest.SampleWorkspace, planning.SprintInput{Name: "Sprint 1"})
	b, _ := svc.CreateSprint(ctx, store, apptest.SampleWorkspace, planning.SprintInput{Name: "Sprint 2"})
	err := svc.StartSprint(ctx, store, apptest.SampleWorkspace, a.ID, "2026-11-14", "2026-11-28")
	if message(err) != "開始日は今日以前にしてください" {
		t.Errorf("tomorrow: %v", err)
	}
	if err := svc.StartSprint(ctx, store, apptest.SampleWorkspace, a.ID, "2026-11-13", "2026-11-27"); err != nil { // 0 件でも開始できる
		t.Fatal(err)
	}
	if got := lastSummary(rec); got != "スプリント「Sprint 1」を開始" {
		t.Errorf("summary = %q", got)
	}
	if got := sprints(t, svc)[0]; got.State != dp.Active || got.Start != "2026-11-13" || got.End != "2026-11-27" {
		t.Errorf("stored = %+v", got)
	}
	err = svc.StartSprint(ctx, store, apptest.SampleWorkspace, b.ID, "2026-11-13", "2026-11-27")
	if message(err) != "実行中のスプリントがあります。先に完了してください" {
		t.Errorf("second active: %v", err)
	}
	if err := svc.StartSprint(ctx, store, apptest.SampleWorkspace, a.ID, "2026-11-13", "2026-11-27"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("already active: %v", err)
	}
}
