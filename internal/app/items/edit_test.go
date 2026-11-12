package items_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// recorder は、Write に渡したアクティビティの 1 行を覚える。
type recorder struct {
	*apptest.Stores
	summaries []string
}

func (r *recorder) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	r.summaries = append(r.summaries, op.Summary)
	return r.Stores.Write(ctx, id, op, fn)
}

func setupRec(t *testing.T) (items.Service, *recorder) {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore(store)
	if err := apptest.Sample(context.Background(), s, store); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{Stores: s}
	return items.Service{Stores: rec, IDs: ids, Identity: apptest.Identity{Login: "hanako"}}, rec
}

// logLen は、アイテムのログの行数。何も保存しなかったことを確かめるのに使う。
func logLen(t *testing.T, s port.Stores, id item.ID) int {
	t.Helper()
	n := 0
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: id})
		n = len(log)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func last(r *recorder) string {
	if len(r.summaries) == 0 {
		return ""
	}
	return r.summaries[len(r.summaries)-1]
}

func TestRename(t *testing.T) {
	svc, rec := setupRec(t)
	it, err := svc.Rename(t.Context(), store, apptest.SampleTask, "  ログイン画面を\n 直す ")
	if err != nil || it.Title != "ログイン画面を 直す" {
		t.Fatalf("Rename = %+v, %v", it, err)
	}
	if got := last(rec); got != "HH-2 のタイトルを変更" {
		t.Errorf("summary = %q", got)
	}
}

func TestRenameToTheSameTitleSavesNothing(t *testing.T) {
	svc, rec := setupRec(t)
	before := logLen(t, rec, apptest.SampleTask)
	it, err := svc.Rename(t.Context(), store, apptest.SampleTask, " ログイン画面を作る ")
	if err != nil || it.Title != "ログイン画面を作る" || logLen(t, rec, apptest.SampleTask) != before {
		t.Errorf("Rename = %+v, %v; log %d → %d", it, err, before, logLen(t, rec, apptest.SampleTask))
	}
}

func TestRenameRefusesAnEmptyTitle(t *testing.T) {
	svc, rec := setupRec(t)
	if _, err := svc.Rename(t.Context(), store, apptest.SampleTask, " \n "); message(err) != "タイトルは空にできません" {
		t.Errorf("err = %v", err)
	}
	if len(rec.summaries) != 0 {
		t.Errorf("a refused rename wrote: %v", rec.summaries)
	}
	if _, err := svc.Rename(t.Context(), store, "01NOPE00000000000000000000", "x"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("unknown: %v", err)
	}
}

func TestAddStep(t *testing.T) {
	svc, rec := setupRec(t)
	step, err := svc.AddStep(t.Context(), store, apptest.SampleTask, " 下書き\n を書く ")
	if err != nil {
		t.Fatal(err)
	}
	if step.Kind != item.Step || step.Place.Parent == nil || *step.Place.Parent != apptest.SampleTask ||
		step.Place.Number != 3 || step.Title != "下書き を書く" || step.Place.Project != apptest.SampleProject {
		t.Errorf("step %+v", step)
	}
	if got := last(rec); got != "HH-2 に Step「下書き を書く」を追加" {
		t.Errorf("summary = %q", got)
	}
	second, err := svc.AddStep(t.Context(), store, apptest.SampleTask, "見直す")
	if err != nil || second.Rank <= step.Rank {
		t.Errorf("second step %+v, %v; must come after %q", second, err, step.Rank)
	}
}

func TestAddStepRefuses(t *testing.T) {
	svc, _ := setupRec(t)
	if _, err := svc.AddStep(t.Context(), store, apptest.SampleEpic, "下書き"); message(err) != "Step は Epic の下に置けません" {
		t.Errorf("under epic: %v", err)
	}
	if _, err := svc.AddStep(t.Context(), store, apptest.SampleTask, " "); message(err) != "タイトルを入力してください" {
		t.Errorf("empty: %v", err)
	}
	if _, err := svc.AddStep(t.Context(), store, "01NOPE00000000000000000000", "x"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("unknown parent: %v", err)
	}
}

func TestMoveUpSwapsWithThePreviousSibling(t *testing.T) {
	svc, rec := setupRec(t)
	a, _ := svc.AddStep(t.Context(), store, apptest.SampleTask, "1")
	b, _ := svc.AddStep(t.Context(), store, apptest.SampleTask, "2")
	if err := svc.MoveUp(t.Context(), store, b.ID); err != nil {
		t.Fatal(err)
	}
	if got := last(rec); got != "HH-4 の順位を変更" {
		t.Errorf("summary = %q", got)
	}
	a2, _ := svc.Get(t.Context(), store, a.ID)
	b2, _ := svc.Get(t.Context(), store, b.ID)
	if b2.Rank != a.Rank || a2.Rank != b.Rank {
		t.Errorf("ranks: a %q→%q, b %q→%q", a.Rank, a2.Rank, b.Rank, b2.Rank)
	}
}

func TestMoveUpAtTheTopSavesNothing(t *testing.T) {
	svc, rec := setupRec(t)
	a, _ := svc.AddStep(t.Context(), store, apptest.SampleTask, "1")
	before := logLen(t, rec, a.ID)
	if err := svc.MoveUp(t.Context(), store, a.ID); err != nil || logLen(t, rec, a.ID) != before {
		t.Errorf("MoveUp = %v; log %d → %d", err, before, logLen(t, rec, a.ID))
	}
	// Epic はプロジェクトの直下の兄弟の中で動く。サンプルの Epic は 1 つだけなので先頭。
	if err := svc.MoveUp(t.Context(), store, apptest.SampleEpic); err != nil {
		t.Errorf("epic: %v", err)
	}
}

func TestNextNumberAndFindByNumber(t *testing.T) {
	svc, _ := setupRec(t)
	if n, err := svc.NextNumber(t.Context(), store, apptest.SampleWorkspace); err != nil || n != 3 {
		t.Errorf("NextNumber = %d, %v", n, err)
	}
	it, err := svc.FindByNumber(t.Context(), store, apptest.SampleWorkspace, 2)
	if err != nil || it.ID != apptest.SampleTask {
		t.Errorf("FindByNumber(2) = %+v, %v", it, err)
	}
	if _, err := svc.FindByNumber(t.Context(), store, apptest.SampleWorkspace, 9); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("FindByNumber(9): %v", err)
	}
	if _, err := svc.NextNumber(t.Context(), store, "01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("NextNumber(unknown): %v", err)
	}
}
