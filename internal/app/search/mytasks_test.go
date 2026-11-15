package search

import (
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// 優先度の内部 ID(DefaultSettings の順で High は Medium の 1 つ前、Low は 1 つ後)。
const (
	priorityHigh change.ID = "01SAMPLEVAL000000000000007"
	priorityLow  change.ID = "01SAMPLEVAL000000000000009"
)

func myTasksOf(t *testing.T, f fixture, filter string) Result {
	t.Helper()
	res, err := f.svc.MyTasks(t.Context(), filter)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// mine は、担当が taro のアイテム。
func mine(id item.ID, n int, kind item.Kind, title string) item.Item {
	it := task(id, n, kind, nil, title)
	it.Assignee = "taro"
	return it
}

func TestMyTasksBase(t *testing.T) {
	f := newFixture(t)
	must(t, apptest.Sample(t.Context(), f.stores, store1))
	f.svc.Identity = loginByStore{store1: "taro", store2: "taro"}
	todo := mine(item3, 3, item.Task, "未着手の Task")
	step := mine(item4, 4, item.Step, "進行中の Step")
	step.Status = apptest.SampleStatusInProgress
	epic := mine(item5, 5, item.Epic, "Epic")
	done := mine(item6, 6, item.Task, "完了の Task")
	done.Status = apptest.SampleStatusDone
	other := mine("01TESTIT000000000000000007", 7, item.Task, "hanako の Task")
	other.Assignee = "hanako"
	bug := mine("01TESTIT000000000000000008", 8, item.Bug, "担当なしの Bug")
	bug.Assignee = ""
	for _, it := range []item.Item{todo, step, epic, done, other, bug} {
		addItem(t, f.stores, it)
	}
	res := myTasksOf(t, f, "")
	if got, want := rowDisplays(res), []string{"S1/HH-3", "S1/HH-4"}; !reflect.DeepEqual(got, want) {
		t.Errorf("rows = %v, want %v", got, want)
	}
}

func TestMyTasksFilterAnds(t *testing.T) {
	f := newFixture(t)
	must(t, apptest.Sample(t.Context(), f.stores, store1))
	hi := mine(item3, 3, item.Task, "high")
	hi.Priority = priorityHigh
	lo := mine(item4, 4, item.Task, "low")
	lo.Priority = priorityLow
	addItem(t, f.stores, hi)
	addItem(t, f.stores, lo)
	res := myTasksOf(t, f, "priority:high")
	if got, want := rowDisplays(res), []string{"S1/HH-3"}; !reflect.DeepEqual(got, want) {
		t.Errorf("rows = %v, want %v", got, want)
	}
}

func TestMyTasksIgnoresSortAndGroup(t *testing.T) {
	f := newFixture(t)
	must(t, apptest.Sample(t.Context(), f.stores, store1))
	addItem(t, f.stores, mine(item3, 3, item.Task, "b"))
	addItem(t, f.stores, mine(item4, 4, item.Task, "a"))
	want := rowDisplays(myTasksOf(t, f, ""))
	got := rowDisplays(myTasksOf(t, f, "sort:title group:status"))
	if !reflect.DeepEqual(got, want) || len(got) != 2 {
		t.Errorf("rows = %v, want %v", got, want)
	}
}

func TestMyTasksFilterError(t *testing.T) {
	f := newFixture(t)
	must(t, apptest.Sample(t.Context(), f.stores, store1))
	addItem(t, f.stores, mine(item3, 3, item.Task, "x"))
	res := myTasksOf(t, f, "status:")
	if want := []string{"「status:」に値がありません"}; !reflect.DeepEqual(res.Errors, want) || len(res.Rows) != 0 {
		t.Errorf("Errors = %v, Rows = %d", res.Errors, len(res.Rows))
	}
}

func TestMyTasksDeleted(t *testing.T) {
	f := newFixture(t)
	must(t, apptest.Sample(t.Context(), f.stores, store1))
	addItem(t, f.stores, mine(item3, 3, item.Task, "消す"))
	must(t, apptest.DeleteItems(t.Context(), f.stores, store1, item3))
	if res := myTasksOf(t, f, ""); len(res.Rows) != 0 {
		t.Errorf("削除済みは出ない: %v", rowDisplays(res))
	}
	res := myTasksOf(t, f, "is:deleted")
	if got, want := rowDisplays(res), []string{"S1/HH-3"}; !reflect.DeepEqual(got, want) || res.Rows[0].DaysLeft == nil {
		t.Errorf("rows = %v, DaysLeft = %v", got, res.Rows)
	}
}
