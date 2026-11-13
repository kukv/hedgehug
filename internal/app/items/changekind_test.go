package items_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
)

func TestChangeKindAmongStoryTaskBugKeepsEverythingElse(t *testing.T) {
	svc, rec := setupRec(t)
	step, _ := svc.AddStep(t.Context(), store, apptest.SampleTask, "下書き") // HH-3
	before := storedItem(t, rec, apptest.SampleTask)
	n := logLen(t, rec, apptest.SampleTask)
	if err := svc.ChangeKind(t.Context(), store, apptest.SampleTask, item.Bug); err != nil {
		t.Fatal(err)
	}
	if got := last(rec); got != "HH-2 の種類を Bug に変更" {
		t.Errorf("summary = %q", got)
	}
	after := storedItem(t, rec, apptest.SampleTask)
	if after.Kind != item.Bug || !samePlace(after.Place, before.Place) || after.Rank != before.Rank || after.Status != before.Status || logLen(t, rec, apptest.SampleTask) != n+1 {
		t.Errorf("after = %+v (before %+v), log %d → %d", after, before, n, logLen(t, rec, apptest.SampleTask))
	}
	if kid := storedItem(t, rec, step.ID); kid.Place.Parent == nil || *kid.Place.Parent != apptest.SampleTask {
		t.Errorf("Step を持っていても変えられ、Step は残る: %+v", kid.Place)
	}
}

func TestChangeKindOfAStepPromotesItToTheEnd(t *testing.T) {
	svc, rec := setupRec(t)
	step, _ := svc.AddStep(t.Context(), store, apptest.SampleTask, "下書き")                        // HH-3
	other := mustCreate(t, svc, apptest.SampleProject, ptr(apptest.SampleEpic), item.Bug, "不具合") // HH-4
	if err := svc.ChangeKind(t.Context(), store, step.ID, item.Story); err != nil {
		t.Fatal(err)
	}
	if got := last(rec); got != "HH-3 の種類を Story に変更" {
		t.Errorf("summary = %q", got)
	}
	got := storedItem(t, rec, step.ID)
	if got.Kind != item.Story || got.Place.Parent == nil || *got.Place.Parent != apptest.SampleEpic || got.Place.Number != 3 || got.Rank <= other.Rank {
		t.Errorf("promoted = %+v, rank %q (last %q)", got.Place, got.Rank, other.Rank)
	}
	ops := lastOps(t, rec, step.ID, 3) // kind・place・rank が同じ操作
	if ops[0] != ops[1] || ops[1] != ops[2] {
		t.Errorf("ops = %v", ops)
	}
}

func TestChangeKindOfAStepShowsOneHistoryLine(t *testing.T) {
	svc, rec := setupRec(t)
	step, _ := svc.AddStep(t.Context(), store, apptest.SampleTask, "下書き")
	if err := svc.ChangeKind(t.Context(), store, step.ID, item.Task); err != nil {
		t.Fatal(err)
	}
	want := []string{"種類を Step から Task に変更", "作成"}
	if got := historyTexts(t, rec, step.ID); !slices.Equal(got, want) {
		t.Errorf("history = %q, want %q", got, want)
	}
}

func TestChangeKindOfAStepIgnoresDeletedSiblingsForTheLastRank(t *testing.T) {
	svc, rec := setupRec(t)
	step, _ := svc.AddStep(t.Context(), store, apptest.SampleTask, "下書き")
	live := mustCreate(t, svc, apptest.SampleProject, ptr(apptest.SampleEpic), item.Bug, "残る")
	gone := mustCreate(t, svc, apptest.SampleProject, ptr(apptest.SampleEpic), item.Bug, "消す")
	markDeleted(t, rec, gone.ID, true)
	if err := svc.ChangeKind(t.Context(), store, step.ID, item.Story); err != nil {
		t.Fatal(err)
	}
	// 削除済みの兄弟の順位と比べていたら、消した子より後ろに入る。
	if got := storedItem(t, rec, step.ID); got.Rank <= live.Rank || got.Rank > gone.Rank {
		t.Errorf("rank = %q (live %q, gone %q)", got.Rank, live.Rank, gone.Rank)
	}
}

func TestChangeKindRefuses(t *testing.T) {
	svc, rec := setupRec(t)
	step, _ := svc.AddStep(t.Context(), store, apptest.SampleTask, "下書き")
	gone := mustCreate(t, svc, apptest.SampleProject, ptr(apptest.SampleEpic), item.Task, "消した Task")
	markDeleted(t, rec, gone.ID, false) // 親と一緒に削除されたもの(root: true なら保存で削除を取り消す)
	cases := []struct {
		name string
		id   item.ID
		kind item.Kind
	}{
		{"Epic は変えられない", apptest.SampleEpic, item.Story},
		{"Epic にはできない", apptest.SampleTask, item.Epic},
		{"Step にはできない", apptest.SampleTask, item.Step},
		{"同じ種類", apptest.SampleTask, item.Task},
		{"Step から Step", step.ID, item.Step},
		{"削除済み", gone.ID, item.Bug},
		{"ない", "01NOPE00000000000000000000", item.Bug},
	}
	for _, c := range cases {
		if err := svc.ChangeKind(t.Context(), store, c.id, c.kind); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	if got := storedItem(t, rec, apptest.SampleTask); got.Kind != item.Task {
		t.Errorf("断ったら変えない: %+v", got)
	}
}
