package item_test

import (
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

func ids(its []item.Item) []item.ID {
	out := make([]item.ID, len(its))
	for i, it := range its {
		out[i] = it.ID
	}
	return out
}

func TestFromLogReadsTheDeletionMark(t *testing.T) {
	at := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	create := item.Create(item.Item{ID: "01A", Kind: item.Epic, Rank: "V", Title: "認証"})
	root := true
	log := change.Stamp(nil, []change.Draft{create}, change.Meta{Op: "o1", At: at, Actor: change.User("taro")}, func() change.ID { return "r1" })
	it, ok := item.FromLog(log)
	if !ok || it.Deleted != nil {
		t.Fatalf("live item = %+v, %v", it, ok)
	}
	del := change.Stamp(log, []change.Draft{change.NewDeleteEntity(root)}, change.Meta{Op: "o2", At: at.Add(time.Hour), Actor: change.User("taro")}, func() change.ID { return "r2" })
	it, ok = item.FromLog(append(log, del...))
	if !ok || it.Deleted == nil || it.Deleted.Op != "o2" || !it.Deleted.At.Equal(at.Add(time.Hour)) || !it.Deleted.Root {
		t.Errorf("deleted item = %+v, %v", it.Deleted, ok)
	}
	// 削除の行はほかの項目を変えない(タイトルなどは削除のあとも読める)。
	if it.Title != "認証" || it.Rank != "V" {
		t.Errorf("deleted item lost its values: %+v", it)
	}
}

func TestLive(t *testing.T) {
	gone := &change.Deletion{Op: "o"}
	its := []item.Item{{ID: "01A"}, {ID: "01B", Deleted: gone}, {ID: "01C"}}
	got := item.Live(its)
	if len(got) != 2 || got[0].ID != "01A" || got[1].ID != "01C" {
		t.Errorf("Live = %v", ids(got))
	}
	if len(its) != 3 {
		t.Error("Live changed its argument")
	}
	if got := item.Live(nil); got == nil || len(got) != 0 {
		t.Errorf("Live(nil) = %#v, want an empty slice", got)
	}
}

func TestDescendants(t *testing.T) {
	p := func(id item.ID) *item.ID { return &id }
	gone := &change.Deletion{Op: "o"}
	all := []item.Item{
		{ID: "01E", Kind: item.Epic, Rank: "V"},
		{ID: "01T2", Kind: item.Task, Rank: "W", Place: item.Place{Parent: p("01E")}},
		{ID: "01T1", Kind: item.Task, Rank: "V", Place: item.Place{Parent: p("01E")}},
		{ID: "01S2", Kind: item.Step, Rank: "W", Place: item.Place{Parent: p("01T1")}, Deleted: gone},
		{ID: "01S1", Kind: item.Step, Rank: "V", Place: item.Place{Parent: p("01T1")}},
		{ID: "01X", Kind: item.Epic, Rank: "W"},
		{ID: "01XT", Kind: item.Task, Rank: "V", Place: item.Place{Parent: p("01X")}},
	}
	// 深さ優先で、兄弟は並びの順。削除済み(01S2)も含み、別の Epic の下は含まない。
	want := []item.ID{"01T1", "01S1", "01S2", "01T2"}
	if got := ids(item.Descendants("01E", all)); !slices.Equal(got, want) {
		t.Errorf("Descendants(01E) = %v, want %v", got, want)
	}
	if got := item.Descendants("01S1", all); len(got) != 0 {
		t.Errorf("Descendants(leaf) = %v", ids(got))
	}
	// 渡した順は変えない。
	if all[1].ID != "01T2" {
		t.Error("Descendants reordered its argument")
	}
	// 親をたどると輪になっていても終わる(同期で入れ違った親)。
	loop := []item.Item{
		{ID: "01A", Place: item.Place{Parent: p("01B")}},
		{ID: "01B", Place: item.Place{Parent: p("01A")}},
	}
	if got := ids(item.Descendants("01A", loop)); !slices.Equal(got, []item.ID{"01B"}) {
		t.Errorf("Descendants in a loop = %v", got)
	}
}
