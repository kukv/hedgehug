package repair_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/repair"
)

// node は、プロジェクト p1 のアイテム。parent が空なら親なし。
func node(id string, n int, kind item.Kind, parent string) item.Item {
	it := item.Item{ID: item.ID(id), Kind: kind, Place: item.Place{Project: "p1", Number: n}}
	if parent != "" {
		p := item.ID(parent)
		it.Place.Parent = &p
	}
	return it
}

func TestCycleOfTwo(t *testing.T) {
	items := []item.Item{node("b", 7, item.Task, "a"), node("a", 5, item.Story, "b"), node("e", 1, item.Epic, "")}
	ring, ok := repair.Cycle(items)
	if !ok || !reflect.DeepEqual(ring, []item.ID{"a", "b"}) {
		t.Fatalf("Cycle = %v %v, want [a b]", ring, ok)
	}
	if m := repair.Mover(ring, items); m != "b" {
		t.Errorf("Mover = %s, want b", m)
	}
}

// 番号が同じなら、内部 ID の辞書順であとのものを動かす。
func TestCycleMoverSameNumber(t *testing.T) {
	items := []item.Item{node("a", 3, item.Task, "b"), node("b", 3, item.Task, "a")}
	if m := repair.Mover([]item.ID{"a", "b"}, items); m != "b" {
		t.Errorf("Mover = %s, want b", m)
	}
}

// 削除済みのアイテムを通る輪は、輪とみなさない。輪へ続く尾も輪に入れない。
func TestCycleSkipsDeleted(t *testing.T) {
	dead := node("b", 2, item.Task, "a")
	dead.Deleted = &change.Deletion{Op: "op"}
	items := []item.Item{node("a", 1, item.Task, "b"), dead}
	if ring, ok := repair.Cycle(items); ok {
		t.Errorf("Cycle = %v, want none", ring)
	}
	tail := []item.Item{node("t", 1, item.Step, "x"), node("x", 2, item.Task, "y"), node("y", 3, item.Task, "x")}
	if ring, ok := repair.Cycle(tail); !ok || !reflect.DeepEqual(ring, []item.ID{"x", "y"}) {
		t.Errorf("Cycle = %v %v, want [x y]", ring, ok)
	}
}

// 輪が 2 つあれば、内部 ID の最も小さいものを含む輪を先に返す。自分が親の 1 件も輪。
func TestCycleFirstByID(t *testing.T) {
	items := []item.Item{node("d", 1, item.Task, "c"), node("c", 2, item.Task, "d"), node("z", 3, item.Task, "b"), node("b", 4, item.Task, "z")}
	if ring, ok := repair.Cycle(items); !ok || !reflect.DeepEqual(ring, []item.ID{"b", "z"}) {
		t.Errorf("Cycle = %v %v, want [b z]", ring, ok)
	}
	self := []item.Item{node("s", 1, item.Task, "s")}
	if ring, ok := repair.Cycle(self); !ok || !reflect.DeepEqual(ring, []item.ID{"s"}) {
		t.Errorf("Cycle = %v %v, want [s]", ring, ok)
	}
}

func TestDestinationOf(t *testing.T) {
	e1, e2 := node("e1", 1, item.Epic, ""), node("e2", 2, item.Epic, "")
	e1.Rank, e2.Rank = "V", "N"
	other := node("e0", 3, item.Epic, "")
	other.Place.Project = project.ID("p2")
	dead := node("e3", 4, item.Epic, "")
	dead.Rank, dead.Deleted = "A", &change.Deletion{Op: "op"}
	broken := node("e4", 5, item.Epic, "x")
	broken.Rank = "B"
	mover := node("t", 6, item.Task, "x")
	items := []item.Item{e1, e2, other, dead, broken, mover}
	got := repair.DestinationOf(mover, items)
	if got.NeedEpic || got.Parent == nil || *got.Parent != "e2" {
		t.Errorf("DestinationOf = %+v, want e2", got)
	}
	if !reflect.DeepEqual(items, []item.Item{e1, e2, other, dead, broken, mover}) {
		t.Errorf("items were reordered")
	}
	if got := repair.DestinationOf(mover, []item.Item{other, dead, broken, mover}); !got.NeedEpic || got.Parent != nil {
		t.Errorf("DestinationOf without epic = %+v, want NeedEpic", got)
	}
	if got := repair.DestinationOf(broken, items); got.NeedEpic || got.Parent != nil {
		t.Errorf("DestinationOf epic = %+v, want project root", got)
	}
}

// 並び順の値が同じ Epic は、作成の時刻の早い方が先。
func TestDestinationOfSameRank(t *testing.T) {
	e1, e2 := node("e1", 1, item.Epic, ""), node("e2", 2, item.Epic, "")
	e1.Rank, e2.Rank = "V", "V"
	e1.Created.At = time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	e2.Created.At = time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	got := repair.DestinationOf(node("s", 3, item.Step, "x"), []item.Item{e1, e2})
	if got.Parent == nil || *got.Parent != "e2" {
		t.Errorf("DestinationOf = %+v, want e2", got)
	}
}

func TestEpicID(t *testing.T) {
	p := project.ID("01ABCDEFGHJKMNPQRSTVWXYZ00")
	if got := repair.EpicID(p, func(change.ID) bool { return false }); got != "R1ABCDEFGHJKMNPQRSTVWXYZ00" {
		t.Errorf("EpicID = %s", got)
	}
	used := func(id change.ID) bool { return id == "R1ABCDEFGHJKMNPQRSTVWXYZ00" }
	if got := repair.EpicID(p, used); got != "S1ABCDEFGHJKMNPQRSTVWXYZ00" {
		t.Errorf("EpicID = %s", got)
	}
	if got := repair.EpicID(p, func(change.ID) bool { return true }); got != "" {
		t.Errorf("EpicID all used = %s, want empty", got)
	}
}
