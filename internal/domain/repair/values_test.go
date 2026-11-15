package repair_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// status は、ステータス id(名前 name・分類 c)を足す行。
func status(id, name string, c workspace.Category) []change.Draft {
	l := workspace.List{Kind: workspace.ListStatuses}
	return workspace.ValueDrafts(l, workspace.Value{ID: change.ID(id), Name: name, Color: "gray", Category: c, Rank: "a" + id})
}

// priority は、優先度 id(名前 name)を足す行。
func priority(id, name string) []change.Draft {
	l := workspace.List{Kind: workspace.ListPriorities}
	return workspace.ValueDrafts(l, workspace.Value{ID: change.ID(id), Name: name, Color: "gray", Rank: "a" + id})
}

// valueLog は、ステータス Todo(未着手)・Doing・Review(進行中)・Done(完了)・Cancel(中止)と、優先度 High・Medium を持つ
// ワークスペースのログ。初期ステータスは Todo、初期優先度は Medium、分類の既定は未着手・完了・中止だけ。
func valueLog() []change.Record {
	var ds []change.Draft
	ds = append(ds, status("s-todo", "Todo", workspace.CategoryTodo)...)
	ds = append(ds, status("s-doing", "Doing", workspace.CategoryDoing)...)
	ds = append(ds, status("s-review", "Review", workspace.CategoryDoing)...)
	ds = append(ds, status("s-done", "Done", workspace.CategoryDone)...)
	ds = append(ds, status("s-cancel", "Cancel", workspace.CategoryCanceled)...)
	ds = append(ds, priority("p-high", "High")...)
	ds = append(ds, priority("p-mid", "Medium")...)
	ds = append(ds,
		change.NewSet("initial_status", nil, "s-todo"), change.NewSet("initial_priority", nil, "p-mid"),
		change.NewSet("category_default/todo", nil, "s-todo"), change.NewSet("category_default/done", nil, "s-done"),
		change.NewSet("category_default/canceled", nil, "s-cancel"),
	)
	return put(wsBase(), "v0", day, "taro", ds...)
}

// opRows は、ops の op の行の ID。
func opRows(log []change.Record, ops ...string) []change.ID {
	var out []change.ID
	for _, r := range log {
		for _, op := range ops {
			if r.Op == change.ID(op) {
				out = append(out, r.ID)
			}
		}
	}
	return out
}

func TestReasonOf(t *testing.T) {
	delReview := change.NewDelete("statuses/s-review")
	tests := []struct {
		name string
		log  []change.Record
		f    string
		want repair.RestoreReason
		ok   bool
	}{
		{
			"初期ステータス", put(valueLog(), "d1", day, "hanako",
				change.NewSet("initial_status", "s-todo", "s-review"), delReview),
			"statuses/s-review",
			repair.RestoreReason{Field: "initial_status"},
			true,
		},
		{
			"進行中の既定", put(valueLog(), "d1", day, "hanako",
				change.NewSet("category_default/doing", nil, "s-review"), delReview),
			"statuses/s-review",
			repair.RestoreReason{Field: "category_default/doing"},
			true,
		},
		{
			"初期優先度", put(valueLog(), "d1", day, "hanako",
				change.NewSet("initial_priority", "p-mid", "p-high"), change.NewDelete("priorities/p-high")),
			"priorities/p-high",
			repair.RestoreReason{Field: "initial_priority"},
			true,
		},
		{
			"進行中が 0 個", put(valueLog(), "d1", day, "hanako", delReview, change.NewDelete("statuses/s-doing")),
			"statuses/s-review",
			repair.RestoreReason{Category: workspace.CategoryDoing},
			true,
		},
		{
			"どれでもない", put(valueLog(), "d1", day, "hanako", delReview),
			"statuses/s-review",
			repair.RestoreReason{},
			false,
		},
		{
			"削除されていない", put(valueLog(), "d1", day, "hanako", change.NewSet("initial_status", "s-todo", "s-review")),
			"statuses/s-review",
			repair.RestoreReason{},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := repair.ReasonOf(tt.log, tt.f)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("got (%+v, %v), want (%+v, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestValueRestoresReferenced(t *testing.T) {
	log := put(valueLog(), "d1", day, "hanako", change.NewSet("initial_status", "s-todo", "s-review"))
	log = put(log, "d2", day, "hanako", change.NewDelete("statuses/s-review"))
	want := []repair.Overlap{{Op: "d2", F: "statuses/s-review"}}
	if got := repair.ValueRestores(log, repair.Facts{}); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestValueRestoresKeepsPointedF(t *testing.T) {
	// d2 は「Done」(完了の最後。指されていない)と「Review」(初期ステータス)を削除する。f の辞書順では「Done」が先。
	log := put(valueLog(), "d1", day, "hanako",
		change.NewSet("initial_status", "s-todo", "s-review"), change.NewSet("category_default/done", "s-done", "s-todo"))
	log = put(log, "d2", day, "jiro", change.NewDelete("statuses/s-done"), change.NewDelete("statuses/s-review"))
	want := []repair.Overlap{{Op: "d2", F: "statuses/s-review"}}
	got := repair.ValueRestores(log, repair.Facts{})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("1 つの op が 2 つの値を削除したら、取り消しの理由の先の f(指されている方): got %+v, want %+v", got, want)
	}
	if r, ok := repair.ReasonOf(log, got[0].F); !ok || r.Field != "initial_status" {
		t.Fatalf("返した f の理由が読める: (%+v, %v)", r, ok)
	}
}

func TestValueRestoresReferencedDeletedTwice(t *testing.T) {
	log := put(valueLog(), "d1", day, "hanako", change.NewSet("initial_status", "s-todo", "s-review"))
	log = put(log, "d3", day, "hanako", change.NewDelete("statuses/s-review"))
	log = put(log, "d2", day, "jiro", change.NewDelete("statuses/s-review"))
	want := []repair.Overlap{{Op: "d2", F: "statuses/s-review"}, {Op: "d3", F: "statuses/s-review"}}
	if got := repair.ValueRestores(log, repair.Facts{}); !reflect.DeepEqual(got, want) {
		t.Fatalf("2 つの削除がどちらも残っていれば、両方を取り消す: got %+v, want %+v", got, want)
	}
}

// emptyDoneLog は、完了が D1(既定)・D2・D3 のログに、remote の側(t1・t2)で既定を D2 にして D1 を削除し、
// 自分の側(o1・o2)で D2 を未着手にして D3 を削除したもの。完了に、いまあるステータスがない。
// remote の側の削除の方が at があと。
func emptyDoneLog() ([]change.Record, repair.Facts) {
	var ds []change.Draft
	ds = append(ds, status("s-todo", "Todo", workspace.CategoryTodo)...)
	ds = append(ds, status("d1", "Done", workspace.CategoryDone)...)
	ds = append(ds, status("d2", "Closed", workspace.CategoryDone)...)
	ds = append(ds, status("d3", "Shipped", workspace.CategoryDone)...)
	ds = append(ds, change.NewSet("initial_status", nil, "s-todo"), change.NewSet("category_default/done", nil, "d1"))
	log := put(wsBase(), "v0", day, "taro", ds...)
	log = put(log, "t1", day.Add(time.Hour), "hanako", change.NewSet("category_default/done", "d1", "d2"))
	log = put(log, "t2", day.Add(3*time.Hour), "hanako", change.NewDelete("statuses/d1"))
	log = put(log, "o1", day.Add(time.Hour), "taro", change.NewSet("statuses/d2/category", "done", "todo"))
	log = put(log, "o2", day.Add(2*time.Hour), "taro", change.NewDelete("statuses/d3"))
	return log, facts(opRows(log, "o1", "o2"), opRows(log, "t1", "t2"))
}

func TestValueRestoresEmptyCategory(t *testing.T) {
	log, f := emptyDoneLog()
	pointed := put(log, "o3", day.Add(4*time.Hour), "taro", change.NewSet("category_default/done", "d2", "d3"))
	tests := []struct {
		name string
		log  []change.Record
		f    repair.Facts
		want []repair.Overlap
	}{
		{"Facts があれば自分の側の削除", log, f, []repair.Overlap{{Op: "o2", F: "statuses/d3"}}},
		{"Facts が空なら at があとの削除", log, repair.Facts{}, []repair.Overlap{{Op: "t2", F: "statuses/d1"}}},
		{
			"既定が指して取り消すものがあれば、分類の分は足さない", pointed,
			repair.Facts{},
			[]repair.Overlap{{Op: "o2", F: "statuses/d3"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := repair.ValueRestores(tt.log, tt.f); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestValueRestoresEmptyCategoryTieByID(t *testing.T) {
	log := put(valueLog(), "a1", day, "taro", status("s-done2", "Closed", workspace.CategoryDone)...)
	log = put(log, "a2", day, "taro", change.NewSet("category_default/done", "s-done", "s-todo"))
	log = put(log, "e2", day, "hanako", change.NewDelete("statuses/s-done"))
	log = put(log, "e1", day, "jiro", change.NewDelete("statuses/s-done2"))
	want := []repair.Overlap{{Op: "e1", F: "statuses/s-done2"}}
	if got := repair.ValueRestores(log, repair.Facts{}); !reflect.DeepEqual(got, want) {
		t.Fatalf("at が同じなら行の ID の大きい方: got %+v, want %+v", got, want)
	}
}

func TestValueRestoresEachCategory(t *testing.T) {
	log := put(valueLog(), "d1", day, "hanako", change.NewDelete("statuses/s-done"))
	log = put(log, "d2", day, "jiro", change.NewSet("category_default/done", "s-done", "s-todo"))
	log = put(log, "d3", day, "jiro", change.NewDelete("statuses/s-cancel"))
	log = put(log, "d4", day, "jiro", change.NewSet("category_default/canceled", "s-cancel", "s-todo"))
	want := []repair.Overlap{{Op: "d1", F: "statuses/s-done"}, {Op: "d3", F: "statuses/s-cancel"}}
	if got := repair.ValueRestores(log, repair.Facts{}); !reflect.DeepEqual(got, want) {
		t.Fatalf("分類ごとに 1 つ取り消す: got %+v, want %+v", got, want)
	}
}

func TestValueRestoresNothing(t *testing.T) {
	log := put(valueLog(), "d1", day, "hanako", change.NewDelete("statuses/s-review"))
	if got := repair.ValueRestores(log, repair.Facts{}); len(got) != 0 {
		t.Fatalf("指されず、分類も残るなら取り消さない: %+v", got)
	}
	restored := put(valueLog(), "d1", day, "hanako",
		change.NewSet("initial_status", "s-todo", "s-review"), change.NewDelete("statuses/s-review"))
	restored = put(restored, "u1", day, "hanako", change.NewRestore("statuses/s-review", "d1"))
	if got := repair.ValueRestores(restored, repair.Facts{}); len(got) != 0 {
		t.Fatalf("取り消し済みの削除は数えない: %+v", got)
	}
}

func TestFinishedInitial(t *testing.T) {
	toReview := change.NewSet("initial_status", "s-todo", "s-review")
	reset := change.NewSet("initial_status", change.ID("s-review"), change.ID("s-todo"))
	tests := []struct {
		name string
		log  []change.Record
		want change.Draft
		ok   bool
	}{
		{
			"完了", put(valueLog(), "c1", day, "hanako", toReview, change.NewSet("statuses/s-review/category", "doing", "done")),
			reset, true,
		},
		{
			"中止", put(valueLog(), "c1", day, "hanako", toReview, change.NewSet("statuses/s-review/category", "doing", "canceled")),
			reset, true,
		},
		{"進行中", put(valueLog(), "c1", day, "hanako", toReview), change.Draft{}, false},
		{
			"未着手の既定がない", put(valueLog(), "c1", day, "hanako", toReview,
				change.NewSet("statuses/s-review/category", "doing", "done"), change.NewSet("category_default/todo", "s-todo", nil)),
			change.Draft{},
			false,
		},
		{"未着手の既定と同じ", put(valueLog(), "c1", day, "hanako",
			change.NewSet("statuses/s-todo/category", "todo", "done")), change.Draft{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := repair.FinishedInitial(tt.log)
			if !reflect.DeepEqual(got, tt.want) || ok != tt.ok {
				t.Fatalf("got (%+v, %v), want (%+v, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}
