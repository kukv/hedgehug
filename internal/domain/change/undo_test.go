package change_test

import (
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
)

func opRow(id, op string, d change.Draft, prev ...change.ID) change.Record {
	r := rec(id, at(1), "taro", d, prev...)
	r.Op = change.ID(op)
	r.Root = d.Root
	return r
}

func TestUndoDeleteWritesRestore(t *testing.T) {
	log := []change.Record{create(map[string]any{}), opRow("d", "op1", change.NewDeleteEntity(true))}
	got := change.Undo(log, "op1")
	want := []change.Draft{change.NewRestore(change.DeletedField, "op1")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestUndoSetOnlyWhileTip(t *testing.T) {
	s1 := opRow("s1", "op1", change.NewSet("status", "A", "B"))
	got := change.Undo([]change.Record{create(map[string]any{}), s1}, "op1")
	if want := []change.Draft{change.NewSet("status", "B", "A")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tip set: got %+v", got)
	}
	s2 := opRow("s2", "op2", change.NewSet("status", "B", "C"), "s1")
	if got := change.Undo([]change.Record{create(map[string]any{}), s1, s2}, "op1"); len(got) != 0 {
		t.Fatalf("overwritten set must not be undone: %+v", got)
	}
}

func TestUndoAddRemove(t *testing.T) {
	a := opRow("a", "op1", change.NewAdd("systems", "W"))
	r := opRow("r", "op1", change.NewRemove("systems", "X"))
	got := change.Undo([]change.Record{create(map[string]any{}), a, r}, "op1")
	want := []change.Draft{change.NewRemove("systems", "W"), change.NewAdd("systems", "X")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestUndoAddNotTipAfterRemove(t *testing.T) {
	a := opRow("a", "op1", change.NewAdd("systems", "W"))
	r := opRow("r", "op2", change.NewRemove("systems", "W"), "a")
	if got := change.Undo([]change.Record{create(map[string]any{}), a, r}, "op1"); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestUndoSkipsAlreadyRestoredAndOtherKinds(t *testing.T) {
	d := opRow("d", "op1", change.NewDeleteEntity(true))
	res := opRow("r", "op2", change.NewRestore(change.DeletedField, "op1"))
	cm := opRow("m", "op1", change.NewComment("C1", nil, map[string]string{"body": "x"}))
	if got := change.Undo([]change.Record{create(map[string]any{}), d, res, cm}, "op1"); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}
