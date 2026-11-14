package change_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
)

func isFresh(ids ...change.ID) func(change.Record) bool {
	return func(r change.Record) bool { return slices.Contains(ids, r.ID) }
}

func byHanako(r change.Record) bool { return r.By == "hanako" }

func setRow(id string, minute int, by string, to string, prev ...change.ID) change.Record {
	return rec(id, at(minute), by, change.NewSet("status", nil, to), prev...)
}

func TestLostConcurrentSet(t *testing.T) {
	m := setRow("m", 0, "hanako", "A", "c")
	w := setRow("w", 1, "taro", "B", "c")
	got := change.Lost([]change.Record{create(nil), m, w}, isFresh("w"), byHanako)
	if len(got) != 1 || got[0].T != change.Set || got[0].F != "status" || got[0].Mine.ID != "m" || got[0].Winner.ID != "w" {
		t.Errorf("Lost = %+v", got)
	}
}

func TestLostNone(t *testing.T) {
	c := create(nil)
	tests := map[string][]change.Record{
		"sequential":  {c, setRow("m", 0, "hanako", "A", "c"), setRow("w", 1, "taro", "B", "m")},
		"same value":  {c, setRow("m", 0, "hanako", "A", "c"), setRow("w", 1, "taro", "A", "c")},
		"clock order": {c, setRow("m", 1, "hanako", "A", "c"), setRow("w", 0, "taro", "B", "c")},
	}
	for name, log := range tests {
		if got := change.Lost(log, isFresh("w"), byHanako); len(got) != 0 {
			t.Errorf("%s: Lost = %+v; want none", name, got)
		}
	}
}

func TestLostAlreadyLost(t *testing.T) {
	// M は、判定の前から、同時の古い行 O(時計があと)に負けている。
	log := []change.Record{
		create(nil), setRow("m", 0, "hanako", "A", "c"),
		setRow("o", 2, "taro", "C", "c"), setRow("w", 3, "jiro", "D", "c"),
	}
	if got := change.Lost(log, isFresh("w"), byHanako); len(got) != 0 {
		t.Errorf("Lost = %+v; want none", got)
	}
}

func TestLostNotMine(t *testing.T) {
	log := []change.Record{create(nil), setRow("m", 0, "hanako", "A", "c"), setRow("w", 1, "taro", "B", "c")}
	if got := change.Lost(log, isFresh("w"), func(change.Record) bool { return false }); len(got) != 0 {
		t.Errorf("Lost = %+v; want none", got)
	}
}

func TestLostList(t *testing.T) {
	start := create(map[string]any{"systems": []string{"Web", "API"}})
	rm := rec("m", at(0), "hanako", change.NewRemove("systems", "API"), "c")
	ad := rec("w", at(1), "taro", change.NewAdd("systems", "API"), "c")
	got := change.Lost([]change.Record{start, rm, ad}, isFresh("w"), byHanako)
	if len(got) != 1 || got[0].T != change.Add || got[0].F != "systems" || got[0].Mine.ID != "m" || got[0].Winner.ID != "w" {
		t.Errorf("Lost = %+v", got)
	}
	mine := rec("m", at(0), "hanako", change.NewAdd("systems", "API"), "c")
	if got := change.Lost([]change.Record{start, mine, ad}, isFresh("w"), byHanako); len(got) != 0 {
		t.Errorf("both add: Lost = %+v; want none", got)
	}
}

func TestLostComment(t *testing.T) {
	p := note("p", 0, "taro", nil, text("a"), "c")
	m := note("m", 1, "hanako", text("a"), text("x"), "p")
	w := note("w", 2, "taro", text("a"), text("y"), "p")
	got := change.Lost([]change.Record{p, m, w}, isFresh("w"), byHanako)
	if len(got) != 1 || got[0].T != change.Comment || got[0].F != "K1" || got[0].Mine.ID != "m" || got[0].Winner.ID != "w" {
		t.Errorf("Lost = %+v", got)
	}
	del := note("m", 1, "hanako", text("a"), nil, "p")
	if got := change.Lost([]change.Record{p, del, w}, isFresh("w"), byHanako); len(got) != 0 {
		t.Errorf("my delete: Lost = %+v; want none", got)
	}
	theirDel := note("w", 2, "taro", text("a"), nil, "p")
	if got := change.Lost([]change.Record{p, m, theirDel}, isFresh("w"), byHanako); len(got) != 0 {
		t.Errorf("their delete: Lost = %+v; want none", got)
	}
}

func TestLostWorklog(t *testing.T) {
	wl := func(id string, minute int, by string, to any, prev ...change.ID) change.Record {
		return rec(id, at(minute), by, change.NewWorklog("W1", nil, to), prev...)
	}
	p := wl("p", 0, "taro", text("a"), "c")
	m := wl("m", 1, "hanako", text("x"), "p")
	w := wl("w", 2, "taro", text("y"), "p")
	got := change.Lost([]change.Record{p, m, w}, isFresh("w"), byHanako)
	if len(got) != 1 || got[0].T != change.Worklog || got[0].F != "W1" || got[0].Mine.ID != "m" || got[0].Winner.ID != "w" {
		t.Errorf("Lost = %+v", got)
	}
}

func TestLostIgnoresRowOrder(t *testing.T) {
	log := []change.Record{
		create(nil), setRow("m", 0, "hanako", "A", "c"), setRow("w", 1, "taro", "B", "c"),
		rec("m2", at(0), "hanako", change.NewSet("z", nil, "1"), "c"), rec("w2", at(1), "taro", change.NewSet("z", nil, "2"), "c"),
	}
	reversed := slices.Clone(log)
	slices.Reverse(reversed)
	a := change.Lost(log, isFresh("w", "w2"), byHanako)
	b := change.Lost(reversed, isFresh("w", "w2"), byHanako)
	if len(a) != 2 || !reflect.DeepEqual(a, b) {
		t.Errorf("Lost = %+v, reversed = %+v", a, b)
	}
}

func TestLostSortedByFieldThenID(t *testing.T) {
	c := create(nil)
	m2 := rec("m2", at(0), "hanako", change.NewSet("b", nil, "1"), "c")
	m1 := rec("m1", at(0), "hanako", change.NewSet("a", nil, "1"), "c")
	w2 := rec("w2", at(1), "taro", change.NewSet("b", nil, "2"), "c")
	w1 := rec("w1", at(1), "taro", change.NewSet("a", nil, "2"), "c")
	got := change.Lost([]change.Record{c, m2, m1, w2, w1}, isFresh("w1", "w2"), byHanako)
	if len(got) != 2 || got[0].F != "a" || got[1].F != "b" {
		t.Errorf("Lost = %+v", got)
	}
}

func TestListOf(t *testing.T) {
	log := []change.Record{
		create(map[string]any{"systems": []string{"Web", "API"}}),
		rec("r", at(1), "taro", change.NewRemove("systems", "API"), "c"),
	}
	got := change.ListOf(log, "systems")
	if len(got) != 1 || string(got[0]) != `"Web"` {
		t.Errorf("ListOf = %s; want [\"Web\"]", got)
	}
}
