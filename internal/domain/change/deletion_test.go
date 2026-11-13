package change_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
)

func TestNewDeleteEntityWritesNoFromAndTo(t *testing.T) {
	d := change.NewDeleteEntity(true)
	if d.T != change.Delete || d.F != change.DeletedField || d.From != nil || d.To != nil || d.Root == nil || !*d.Root {
		t.Fatalf("NewDeleteEntity(true) = %+v", d)
	}
	meta := change.Meta{Op: "o", At: time.Date(2026, 11, 13, 9, 0, 0, 0, jst), Actor: change.User("taro")}
	n := 0
	recs := change.Stamp(nil, []change.Draft{change.NewDeleteEntity(true), change.NewDeleteEntity(false)}, meta, func() change.ID { n++; return change.ID("r" + string(rune('0'+n))) })
	want := []string{
		`{"id":"r1","op":"o","at":"2026-11-13T09:00:00.000+09:00","by":"taro","actor":"user","t":"delete","f":"deleted","prev":[],"root":true}`,
		`{"id":"r2","op":"o","at":"2026-11-13T09:00:00.000+09:00","by":"taro","actor":"user","t":"delete","f":"deleted","prev":["r1"],"root":false}`,
	}
	for i, r := range recs {
		b, err := json.Marshal(r)
		if err != nil || string(b) != want[i] {
			t.Errorf("line %d = %s, %v\nwant %s", i, b, err, want[i])
		}
	}
}

func TestDeletionOf(t *testing.T) {
	at := func(h int) time.Time { return time.Date(2026, 11, 13, h, 0, 0, 0, jst) }
	row := func(id, op string, typ change.Type, f string, h int, root bool) change.Record {
		return change.Record{ID: change.ID(id), Op: change.ID(op), At: at(h), By: "taro", Actor: change.User("taro"), T: typ, F: f, Root: &root}
	}
	create := change.Record{ID: "c", Op: "o0", At: at(8), By: "taro", Actor: change.User("taro"), T: change.Create}

	if got := change.DeletionOf([]change.Record{create, row("s", "o1", change.Set, "title", 9, false)}); got != nil {
		t.Errorf("not deleted = %+v", got)
	}
	// 値の削除(f が statuses/…)は、エンティティの削除ではない。
	if got := change.DeletionOf([]change.Record{create, row("d", "o1", change.Delete, "statuses/S1", 9, true)}); got != nil {
		t.Errorf("value delete = %+v", got)
	}
	del := row("d1", "o1", change.Delete, change.DeletedField, 9, true)
	got := change.DeletionOf([]change.Record{create, del})
	if got == nil || got.Op != "o1" || !got.At.Equal(at(9)) || !got.Root {
		t.Fatalf("deleted = %+v", got)
	}
	// root のない行(手で足した行)は root: false として読む。
	del.Root = nil
	if got := change.DeletionOf([]change.Record{create, del}); got == nil || got.Root {
		t.Errorf("no root = %+v", got)
	}
	// 複数あれば at があとの行。行の並びによらない。
	a, b := row("a", "oa", change.Delete, change.DeletedField, 9, true), row("b", "ob", change.Delete, change.DeletedField, 10, false)
	for _, log := range [][]change.Record{{create, a, b}, {b, a, create}} {
		if got := change.DeletionOf(log); got == nil || got.Op != "ob" || got.Root {
			t.Errorf("two deletes = %+v", got)
		}
	}
}

// at が同じなら by(小文字)、さらに同じなら id の大きい行が勝つ(Resolve と同じ順)。
func TestDeletionOfTieBreak(t *testing.T) {
	at := time.Date(2026, 11, 13, 9, 0, 0, 0, jst)
	create := change.Record{ID: "c", Op: "o0", At: at, By: "taro", Actor: change.User("taro"), T: change.Create}
	root := true
	del := func(id, op, by string) change.Record {
		return change.Record{ID: change.ID(id), Op: change.ID(op), At: at, By: by, Actor: change.User(by), T: change.Delete, F: change.DeletedField, Root: &root}
	}
	for _, c := range []struct {
		name string
		a, b change.Record
	}{
		{"by", del("a", "oa", "Hanako"), del("b", "ob", "taro")},
		{"id", del("a", "oa", "taro"), del("b", "ob", "taro")},
	} {
		for _, log := range [][]change.Record{{create, c.a, c.b}, {c.b, c.a, create}} {
			if got := change.DeletionOf(log); got == nil || got.Op != "ob" {
				t.Errorf("tie-break by %s = %+v, want op ob", c.name, got)
			}
		}
	}
}

func TestNewRestore(t *testing.T) {
	d := change.NewRestore(change.DeletedField, "op1")
	if d.T != change.Restore || d.F != change.DeletedField || d.From != nil || d.Root != nil || string(d.To) != `"op1"` {
		t.Fatalf("NewRestore = %+v", d)
	}
}

// restore の行は、f が同じで to が自分の op の削除だけを取り消す。もう一度削除すれば、また削除済みになる(C3a-7)。
func TestDeletionOfSkipsRestored(t *testing.T) {
	at := func(h int) time.Time { return time.Date(2026, 11, 13, h, 0, 0, 0, jst) }
	root := true
	del := func(id, op string, h int) change.Record {
		return change.Record{ID: change.ID(id), Op: change.ID(op), At: at(h), By: "taro", Actor: change.User("taro"), T: change.Delete, F: change.DeletedField, Root: &root}
	}
	restore := func(f, to string) change.Record {
		return change.Record{ID: "r1", Op: "op2", At: at(10), By: "taro", Actor: change.User("taro"), T: change.Restore, F: f, To: change.Value(change.ID(to))}
	}
	d1 := del("d1", "op1", 9)
	if got := change.DeletionOf([]change.Record{d1, restore(change.DeletedField, "op1")}); got != nil {
		t.Fatalf("restored deletion = %+v, want nil", got)
	}
	again := del("d2", "op3", 11)
	if got := change.DeletionOf([]change.Record{d1, restore(change.DeletedField, "op1"), again}); got == nil || got.Op != "op3" {
		t.Fatalf("deleted again = %+v, want op3", got)
	}
	// 別の op の削除を指す restore や、別の f の restore は取り消さない。
	if change.DeletionOf([]change.Record{d1, restore(change.DeletedField, "other")}) == nil {
		t.Error("restore of another op must not undo the deletion")
	}
	if change.DeletionOf([]change.Record{d1, restore("statuses/X", "op1")}) == nil {
		t.Error("restore of another f must not undo the deletion")
	}
	// 取り消された削除より前の、取り消されていない削除は残る。
	older := del("d0", "op0", 8)
	if got := change.DeletionOf([]change.Record{older, d1, restore(change.DeletedField, "op1")}); got == nil || got.Op != "op0" {
		t.Errorf("older live deletion = %+v, want op0", got)
	}
}

func TestRestored(t *testing.T) {
	d := change.Record{ID: "d1", Op: "op1", T: change.Delete, F: "statuses/S"}
	r := change.Record{ID: "r1", Op: "op2", T: change.Restore, F: "statuses/S", To: change.Value(change.ID("op1"))}
	if !change.Restored([]change.Record{d, r})(d) {
		t.Error("restored delete must be reported")
	}
	if change.Restored([]change.Record{d})(d) {
		t.Error("a delete without restore must not be reported")
	}
	if change.Restored([]change.Record{d, r})(r) {
		t.Error("a non-delete row must not be reported")
	}
}
