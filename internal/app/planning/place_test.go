package planning_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/rank"
)

const todo = apptest.SampleStatusTodo

func ref(id item.ID) *item.ID { return &id }

// バックログの枠の HH-2(V)・A(W)・B(X)・C(Y) で、C を A の直前へ。
func TestPlaceItemReorder(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	addWorks(t, svc, []work{{itemC, 3, "Y", todo, "", 1}, {itemA, 4, "W", todo, "", 1}, {itemB, 5, "X", todo, "", 1}})
	n := len(rec.summaries)
	if err := svc.PlaceItem(ctx, store, itemC, nil, ref(itemA), false); err != nil {
		t.Fatal(err)
	}
	if len(rec.summaries) != n+1 || lastSummary(rec) != "HH-3 の順位を変更" {
		t.Errorf("summaries = %v", rec.summaries[n:])
	}
	it, log := getItem(t, svc, itemC)
	if it.Rank != rank.Between("V", "W") {
		t.Errorf("rank = %q", it.Rank)
	}
	if got := lastOp(log); len(got) != 1 || got[0] != "rank" {
		t.Errorf("1 回の保存の行 = %v", got)
	}
	if got := backlogRows(t, svc); !sameIDs(got, apptest.SampleTask, itemC, itemA, itemB) {
		t.Errorf("並び = %v", got)
	}
}

// バックログの HH-2 を S1 の X の直前へ。
func TestPlaceItemAcrossFrames(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	s1, _ := activeSprint(t, svc)
	addWorks(t, svc, []work{{itemX, 3, "A", todo, s1.ID, 1}})
	if err := svc.PlaceItem(ctx, store, apptest.SampleTask, &s1.ID, ref(itemX), false); err != nil {
		t.Fatal(err)
	}
	if lastSummary(rec) != "HH-2 の順位を変更" {
		t.Errorf("summary = %q", lastSummary(rec))
	}
	it, log := getItem(t, svc, apptest.SampleTask)
	if it.Sprint != s1.ID || it.Rank != rank.Between("", "A") {
		t.Errorf("S1 へ: %s/%s", it.Sprint, it.Rank)
	}
	if got := lastOp(log); len(got) != 2 || got[0] != item.SprintField || got[1] != "rank" {
		t.Errorf("1 回の保存の行 = %v", got)
	}
	if sprintChanges(log) != 1 {
		t.Errorf("履歴のスプリントの変更 = %d", sprintChanges(log))
	}
}

// 基準なし: 別の枠なら SetSprint と同じ、同じ枠なら末尾へ、すでに末尾なら何もしない。
func TestPlaceItemEmptyArea(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	s1, _ := activeSprint(t, svc)
	addWorks(t, svc, []work{{itemA, 3, "W", todo, "", 1}, {itemB, 4, "X", todo, "", 1}, {itemY, 5, "Y", todo, s1.ID, 1}})

	if err := svc.PlaceItem(ctx, store, apptest.SampleTask, &s1.ID, nil, false); err != nil {
		t.Fatal(err)
	}
	if lastSummary(rec) != "HH-2 の スプリント を Sprint 1 に変更" {
		t.Errorf("summary = %q", lastSummary(rec))
	}
	it, log := getItem(t, svc, apptest.SampleTask)
	if it.Sprint != s1.ID || it.Rank != rank.After("Y") {
		t.Errorf("S1 へ: %s/%s", it.Sprint, it.Rank)
	}
	if got := lastOp(log); len(got) != 2 || got[0] != item.SprintField || got[1] != "rank" {
		t.Errorf("1 回の保存の行 = %v", got)
	}

	if err := svc.PlaceItem(ctx, store, itemA, nil, nil, false); err != nil {
		t.Fatal(err)
	}
	if lastSummary(rec) != "HH-3 の順位を変更" {
		t.Errorf("summary = %q", lastSummary(rec))
	}
	it, log = getItem(t, svc, itemA)
	if it.Rank != rank.After("X") {
		t.Errorf("末尾へ: %s", it.Rank)
	}
	if got := lastOp(log); len(got) != 1 || got[0] != "rank" {
		t.Errorf("1 回の保存の行 = %v", got)
	}

	n := len(rec.summaries)
	if err := svc.PlaceItem(ctx, store, itemA, nil, nil, false); err != nil {
		t.Fatal(err)
	}
	if len(rec.summaries) != n {
		t.Errorf("末尾で保存した: %v", rec.summaries[n:])
	}
}

// スプリントの枠でも、基準なしで同じ枠なら末尾へ。
func TestPlaceItemEmptyAreaInSprint(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	s1, _ := activeSprint(t, svc)
	addWorks(t, svc, []work{{itemY, 3, "Y", todo, s1.ID, 1}, {itemZ, 4, "Z", todo, s1.ID, 1}})
	if err := svc.PlaceItem(ctx, store, itemY, &s1.ID, nil, false); err != nil {
		t.Fatal(err)
	}
	if lastSummary(rec) != "HH-3 の順位を変更" {
		t.Errorf("summary = %q", lastSummary(rec))
	}
	it, log := getItem(t, svc, itemY)
	if it.Sprint != s1.ID || it.Rank != rank.After("Z") {
		t.Errorf("S1 の末尾へ: %s/%s", it.Sprint, it.Rank)
	}
	if got := lastOp(log); len(got) != 1 || got[0] != "rank" {
		t.Errorf("1 回の保存の行 = %v", got)
	}
}

// 同じ枠で、基準が次の行への直前・前の行への直後・自分自身なら何もしない。
func TestPlaceItemSamePosition(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	addWorks(t, svc, []work{{itemA, 3, "W", todo, "", 1}, {itemB, 4, "X", todo, "", 1}})
	n := len(rec.summaries)
	for name, c := range map[string]struct {
		anchor item.ID
		after  bool
	}{
		"次の行の直前":  {itemB, false},
		"前の行の直後":  {apptest.SampleTask, true},
		"自分自身の直前": {itemA, false},
		"自分自身の直後": {itemA, true},
	} {
		if err := svc.PlaceItem(ctx, store, itemA, nil, ref(c.anchor), c.after); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if len(rec.summaries) != n {
		t.Errorf("同じ位置で保存した: %v", rec.summaries[n:])
	}
	if err := svc.PlaceItem(ctx, store, itemA, nil, ref(itemB), true); err != nil || len(rec.summaries) != n+1 {
		t.Errorf("次の行の直後: %v %v", err, rec.summaries[n:])
	}
}

// 同じ順位が並ぶとき(TestPlacementTies の並び)、群の中には入らない。
func TestPlaceItemTies(t *testing.T) {
	svc, _ := setup(t)
	ctx := t.Context()
	// HH-2(V)・B(W)・C(W。B よりあとに作成)・F(Y)。
	addWorks(t, svc, []work{{itemB, 3, "W", todo, "", 1}})
	addWorks(t, svc, []work{{itemC, 4, "W", todo, "", 1}})
	addWorks(t, svc, []work{{itemD, 5, "Y", todo, "", 1}})
	if got := backlogRows(t, svc); !sameIDs(got, apptest.SampleTask, itemB, itemC, itemD) {
		t.Fatalf("はじめの並び = %v", got)
	}
	// F を C の直前: 群の手前で、V と W の間。
	if err := svc.PlaceItem(ctx, store, itemD, nil, ref(itemC), false); err != nil {
		t.Fatal(err)
	}
	if it, _ := getItem(t, svc, itemD); it.Rank != rank.Between("V", "W") {
		t.Errorf("F の rank = %q, want %q", it.Rank, rank.Between("V", "W"))
	}
	if got := backlogRows(t, svc); !sameIDs(got, apptest.SampleTask, itemD, itemB, itemC) {
		t.Errorf("F を C の直前 = %v", got)
	}
	// HH-2 を C の直後: 群の後ろ。
	if err := svc.PlaceItem(ctx, store, apptest.SampleTask, nil, ref(itemC), true); err != nil {
		t.Fatal(err)
	}
	if got := backlogRows(t, svc); !sameIDs(got, itemD, itemB, itemC, apptest.SampleTask) {
		t.Errorf("HH-2 を C の直後 = %v", got)
	}
}

func TestPlaceItemClosedSprint(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	s1, s2 := activeSprint(t, svc)
	addWorks(t, svc, []work{{itemY, 6, "Y", apptest.SampleStatusDone, s1.ID, 1}}) // 完了で S1 に残る
	if err := svc.CloseSprint(ctx, store, apptest.SampleWorkspace, s1.ID, nil); err != nil {
		t.Fatal(err)
	}
	addWorks(t, svc, []work{{itemX, 3, "A", todo, s2.ID, 1}, {itemA, 4, "W", todo, "", 1}, {itemZ, 5, "X", todo, "", 1}})
	if err := apptest.DeleteItems(ctx, svc.Stores, store, itemZ); err != nil {
		t.Fatal(err)
	}
	n := len(rec.summaries)
	for name, a := range map[string]*item.ID{"基準なし": nil, "基準あり": ref(itemA)} {
		if err := svc.PlaceItem(ctx, store, apptest.SampleTask, &s1.ID, a, false); message(err) != "完了したスプリントには入れられません" {
			t.Errorf("完了したスプリント(%s): %v", name, err)
		}
	}
	if err := svc.PlaceItem(ctx, store, apptest.SampleEpic, nil, ref(itemA), false); !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("Epic: %v", err)
	}
	for name, c := range map[string]struct {
		sprint *change.ID
		anchor item.ID
	}{
		"別の枠の基準":   {nil, itemX},
		"削除済みの基準":  {nil, itemZ},
		"ない基準":     {nil, "01NOSUCHITEM000000000000"},
		"Epic の基準": {nil, apptest.SampleEpic},
		"移動先にない基準": {&s2.ID, itemA},
	} {
		if err := svc.PlaceItem(ctx, store, apptest.SampleTask, c.sprint, ref(c.anchor), false); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if len(rec.summaries) != n {
		t.Errorf("断ったのに保存した: %v", rec.summaries[n:])
	}
	// 完了したスプリントに残る作業項目は、SetSprint と同じく、そのスプリントを枠とみなす。バックログの行の直前へ置くと、
	// スプリントをバックログに変える(順位だけ変えると、完了したスプリントに残ってどの枠にも出ない)。
	if err := svc.PlaceItem(ctx, store, itemY, nil, ref(itemA), false); err != nil {
		t.Fatal(err)
	}
	if it, log := getItem(t, svc, itemY); it.Sprint != "" || len(lastOp(log)) != 2 {
		t.Errorf("完了したスプリントから: sprint %q 行 %v", it.Sprint, lastOp(log))
	}
}

// 別の画面で削除された(root: true)アイテムを並べ替えると、同じ保存で削除を取り消す。
// 同じ位置なら断り、削除済みのまま。
func TestPlaceItemRevives(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	addWorks(t, svc, []work{{itemA, 3, "W", todo, "", 1}, {itemB, 4, "X", todo, "", 1}})
	if err := apptest.DeleteItems(ctx, svc.Stores, store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	n := len(rec.summaries)
	if err := svc.PlaceItem(ctx, store, apptest.SampleTask, nil, ref(itemA), false); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("同じ位置 = %v, want port.ErrNotFound", err)
	}
	if it, _ := getItem(t, svc, apptest.SampleTask); it.Deleted == nil || len(rec.summaries) != n {
		t.Fatalf("同じ位置で取り消した: %+v %v", it.Deleted, rec.summaries[n:])
	}
	if err := svc.PlaceItem(ctx, store, apptest.SampleTask, nil, ref(itemB), true); err != nil {
		t.Fatal(err)
	}
	if it, _ := getItem(t, svc, apptest.SampleTask); it.Deleted != nil || it.Rank != rank.After("X") {
		t.Errorf("取り消し: deleted %+v rank %q", it.Deleted, it.Rank)
	}
	if lastSummary(rec) != "HH-2 の順位を変更" {
		t.Errorf("summary = %q", lastSummary(rec))
	}
}
