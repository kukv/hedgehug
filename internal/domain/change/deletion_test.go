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
