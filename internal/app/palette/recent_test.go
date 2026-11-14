package palette_test

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/palette"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// fakeRecent は port.RecentItems の偽物(apptest はファイル数の上限なので、ここに置く)。
type fakeRecent struct {
	rows    []port.Recent
	keeps   []int         // Touch に渡った keep
	forgets [][]change.ID // Forget に渡った items
}

func (f *fakeRecent) Touch(_ context.Context, store port.StoreID, id change.ID, at time.Time, keep int) error {
	f.keeps = append(f.keeps, keep)
	f.rows = slices.DeleteFunc(f.rows, func(r port.Recent) bool { return r.Item == id })
	f.rows = append(f.rows, port.Recent{Store: store, Item: id, At: at})
	f.sort()
	if len(f.rows) > keep {
		f.rows = f.rows[:keep]
	}
	return nil
}

func (f *fakeRecent) List(context.Context) ([]port.Recent, error) {
	f.sort()
	return slices.Clone(f.rows), nil
}

func (f *fakeRecent) Forget(_ context.Context, items []change.ID) error {
	f.forgets = append(f.forgets, slices.Clone(items))
	f.rows = slices.DeleteFunc(f.rows, func(r port.Recent) bool { return slices.Contains(items, r.Item) })
	return nil
}

// sort は、At の新しい順に並べる。
func (f *fakeRecent) sort() {
	slices.SortStableFunc(f.rows, func(a, b port.Recent) int { return b.At.Compare(a.At) })
}

// remember は、Opened を通さずに記録を足す(n 分前に開いた)。
func (f *fakeRecent) remember(store port.StoreID, id change.ID, n int) {
	f.rows = append(f.rows, port.Recent{Store: store, Item: id, At: clockStart.Add(-time.Duration(n) * time.Minute)})
}

func recent(t *testing.T, f fixture) []palette.RecentItem {
	t.Helper()
	got, err := f.svc.Recent(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func displays(rs []palette.RecentItem) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, string(r.Store)+"/"+r.Display)
	}
	return out
}

// 10 件を Opened で覚える(時計の Now・keep 50 で Touch)→ 新しい 8 件を、いまの ID・タイトル・プロジェクト名で返す。
func TestRecentReturnsNewestEightLiveItems(t *testing.T) {
	f := newFixture(t)
	must(t, apptest.Sample(t.Context(), f.stores, store1))
	for n := 3; n <= 12; n++ {
		id := item.ID(fmt.Sprintf("01TESTIT00000000000000%04d", n))
		addItem(t, f.stores, store1, task(id, apptest.SampleWorkspace, apptest.SampleProject, n, fmt.Sprintf("タスク %d", n)))
		must(t, f.svc.Opened(t.Context(), store1, id))
	}

	if want := slices.Repeat([]int{50}, 10); !reflect.DeepEqual(f.recent.keeps, want) {
		t.Errorf("keeps = %v, want %v", f.recent.keeps, want)
	}
	if got := f.recent.rows[len(f.recent.rows)-1].At; !got.Equal(clockStart) {
		t.Errorf("最初の Touch の At = %v, want %v", got, clockStart)
	}
	got := recent(t, f)
	want := []string{"S1/HH-12", "S1/HH-11", "S1/HH-10", "S1/HH-9", "S1/HH-8", "S1/HH-7", "S1/HH-6", "S1/HH-5"}
	if !reflect.DeepEqual(displays(got), want) {
		t.Fatalf("Recent = %v, want %v", displays(got), want)
	}
	first := palette.RecentItem{Store: store1, Item: "01TESTIT000000000000000012", Display: "HH-12", Title: "タスク 12", ProjectTitle: "ログイン"}
	if !reflect.DeepEqual(got[0], first) {
		t.Errorf("Recent[0] = %+v, want %+v", got[0], first)
	}
	if len(f.recent.forgets) != 0 {
		t.Errorf("Forget = %v, want none", f.recent.forgets)
	}
}

// 削除済みのアイテム・削除済みのプロジェクトの下のアイテム・非表示の Store(SyncTargets にあり StoreList にない)の
// アイテムは返さず、表からも消さない(数えないので、後ろの HH-2 が出る)。
func TestRecentSkipsDeletedAndHiddenButKeepsThem(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	must(t, apptest.Sample(ctx, f.stores, store1))
	addItem(t, f.stores, store1, task(gone, apptest.SampleWorkspace, apptest.SampleProject, 13, "削除したもの"))
	must(t, apptest.DeleteItems(ctx, f.stores, store1, gone))
	addProject(t, f.stores, store1, project.Project{ID: project2, Place: project.Place{Workspace: apptest.SampleWorkspace, Number: 2}, Rank: "W", Title: "消したプロジェクト", Status: project.Todo})
	addItem(t, f.stores, store1, task(under, apptest.SampleWorkspace, project2, 14, "消したプロジェクトの下"))
	must(t, apptest.DeleteProject(ctx, f.stores, store1, project2))
	must(t, apptest.SampleTeam(ctx, f.stores, store3))
	f.recent.remember(store1, gone, 1)
	f.recent.remember(store1, under, 2)
	f.recent.remember(store3, apptest.SampleTeamEpic, 3)
	f.recent.remember(store1, apptest.SampleTask, 4)

	if got, want := displays(recent(t, f)), []string{"S1/HH-2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Recent = %v, want %v", got, want)
	}
	if len(f.recent.forgets) != 0 {
		t.Errorf("Forget = %v, want none", f.recent.forgets)
	}
	if len(f.recent.rows) != 4 {
		t.Errorf("rows = %+v, want 4 rows", f.recent.rows)
	}
}

// どれかの SyncTargets の Store の Purged にあるもの(Purged は非表示の S3 にあり、アイテム自体は S1 に削除済みで
// 残っている)と、どの SyncTargets の Store にもないもの(覚えた Store はこのマシンから外した)は、Forget に渡る。
func TestRecentForgetsPurgedAndRemovedStores(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	must(t, apptest.Sample(ctx, f.stores, store1))
	addItem(t, f.stores, store1, task(gone, apptest.SampleWorkspace, apptest.SampleProject, 13, "完全に削除したもの"))
	must(t, apptest.DeleteItems(ctx, f.stores, store1, gone))
	f.stores.AddPurged(store3, port.Purged{Entity: "item", Target: gone, Display: "HH-13", At: clockStart})
	f.recent.remember(store1, gone, 1)
	f.recent.remember("S9", moved, 2)
	f.recent.remember(store1, apptest.SampleTask, 3)

	if got, want := displays(recent(t, f)), []string{"S1/HH-2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Recent = %v, want %v", got, want)
	}
	if want := [][]change.ID{{gone, moved}}; !reflect.DeepEqual(f.recent.forgets, want) {
		t.Errorf("Forget = %v, want %v", f.recent.forgets, want)
	}
}

// 覚えた S1 から S2 へ移した(内部 ID は同じ。S1 には削除済みで残る)→ S2 のいまの ID で返す。
func TestRecentFindsItemMovedToAnotherStore(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	must(t, apptest.Sample(ctx, f.stores, store1))
	must(t, apptest.SampleMobile(ctx, f.stores, store2))
	addItem(t, f.stores, store1, task(moved, apptest.SampleWorkspace, apptest.SampleProject, 12, "移したもの"))
	must(t, apptest.DeleteItems(ctx, f.stores, store1, moved))
	addItem(t, f.stores, store2, task(moved, apptest.SampleMobileWorkspace, apptest.SampleMobileProject, 4, "移したもの"))
	f.recent.remember(store1, moved, 1)

	got := recent(t, f)
	want := []palette.RecentItem{{Store: store2, Item: moved, Display: "MB-4", Title: "移したもの", ProjectTitle: "アプリ"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Recent = %+v, want %+v", got, want)
	}
	if len(f.recent.forgets) != 0 {
		t.Errorf("Forget = %v, want none", f.recent.forgets)
	}
}

// 同じ内部 ID のアイテムが S1・S2 のどちらにもある → 覚えた Store(S2)のものを返す(StoreList の並びより先)。
func TestRecentPrefersRememberedStore(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	must(t, apptest.Sample(ctx, f.stores, store1))
	must(t, apptest.SampleMobile(ctx, f.stores, store2))
	addItem(t, f.stores, store1, task(moved, apptest.SampleWorkspace, apptest.SampleProject, 12, "S1 のもの"))
	addItem(t, f.stores, store2, task(moved, apptest.SampleMobileWorkspace, apptest.SampleMobileProject, 4, "S2 のもの"))
	f.recent.remember(store2, moved, 1)

	if got, want := displays(recent(t, f)), []string{"S2/MB-4"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Recent = %v, want %v", got, want)
	}
}
