package merge

import (
	"bytes"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/schema"
)

// FuzzSymmetric checks that swapping ours and theirs changes nothing.
func FuzzSymmetric(f *testing.F) {
	f.Add("a", "b", "c", "1\n2\n", "1\nx\n", "y\n2\n", int64(1), int64(2))
	mk := func(title, body string, u int64) []byte {
		updated := time.Unix(u%1e9, 0).UTC().Format(time.RFC3339)
		return append(task(updated, "title: "+strconv.Quote(title), "labels: ["+strconv.Quote(title)+"]"), body...)
	}
	f.Fuzz(func(t *testing.T, bt, ot, tt, bb, ob, tb string, ou, tu int64) {
		base, ours, theirs := mk(bt, bb, 0), mk(ot, ob, ou), mk(tt, tb, tu)
		a, err1 := Merge(taskPath, schema.Default(), base, Side{Data: ours, Author: "a"}, Side{Data: theirs, Author: "b"}, LWW)
		b, err2 := Merge(taskPath, schema.Default(), base, Side{Data: theirs, Author: "b"}, Side{Data: ours, Author: "a"}, LWW)
		if (err1 == nil) != (err2 == nil) || !bytes.Equal(a.Data, b.Data) || !sameOverwritten(a.Overwritten, b.Overwritten) || !slices.Equal(a.Notices, b.Notices) {
			t.Fatalf("not symmetric:\n%s\n---\n%s\n%v\n%v", a.Data, b.Data, a.Overwritten, b.Overwritten)
		}
	})
}

// FuzzForwardSymmetric checks the same for a task file of which a side is a
// forward, or removed.
func FuzzForwardSymmetric(f *testing.F) {
	f.Add(uint8(0), uint8(1), "a", "b", int64(1), int64(2))
	f.Add(uint8(1), uint8(1), "a", "a", int64(2), int64(2))
	f.Add(uint8(2), uint8(0), "b", "a", int64(1), int64(1))
	f.Add(uint8(3), uint8(2), "", "", int64(0), int64(0))
	variant := func(v uint8) []byte {
		switch v % 5 {
		case 0:
			return task("2026-10-31T10:00:00Z", "title: x")
		case 1:
			return forwardFile(taskU, storeB)
		case 2:
			return forwardFile(taskU, storeC)
		case 3:
			return task("2026-10-31T11:00:00Z", "title: y")
		}
		return nil
	}
	f.Fuzz(func(t *testing.T, ov, tv uint8, oa, ta string, ot, tt int64) {
		base := task("2026-10-31T09:00:00Z", "title: base")
		o := Side{Data: variant(ov), Author: oa, Time: time.Unix(ot%1e9, 0)}
		th := Side{Data: variant(tv), Author: ta, Time: time.Unix(tt%1e9, 0)}
		a, err1 := Merge(taskPath, schema.Default(), base, o, th, LWW)
		b, err2 := Merge(taskPath, schema.Default(), base, th, o, LWW)
		if (err1 == nil) != (err2 == nil) || !bytes.Equal(a.Data, b.Data) || !sameOverwritten(a.Overwritten, b.Overwritten) || !slices.Equal(a.Notices, b.Notices) {
			t.Fatalf("not symmetric:\n%s\n---\n%s\n%v\n%v", a.Data, b.Data, a.Overwritten, b.Overwritten)
		}
	})
}

// FuzzYAMLSymmetric checks the same for the files merged key by key.
func FuzzYAMLSymmetric(f *testing.F) {
	f.Add("statuses:\n- {name: todo, category: todo}\nprops:\n  p: {values: [a]}\n",
		"# c\nstatuses:\n- {name: todo, category: doing}\n- {name: qa}\nprops:\n  p: {values: [a, b]}\n",
		"statuses:\n- {name: review}\nprops:\n  p: {values: [c]} # p\n  q: 1\n", int64(1), int64(2))
	f.Add("rules:\n- id: r1\n  do: a\nscoped:\n- where: x\n  statuses: [a]\n", "rules:\n- id: r1\n  do: b\n", "rules: []\nscoped:\n- where: x\n  statuses: [b]\n", int64(2), int64(2))
	f.Fuzz(func(t *testing.T, base, ours, theirs string, ou, tu int64) {
		o := Side{Data: []byte(ours), Author: "a", When: whenAt(time.Unix(ou%1e9, 0))}
		th := Side{Data: []byte(theirs), Author: "b", When: whenAt(time.Unix(tu%1e9, 0))}
		a, err1 := Merge("schema.yaml", nil, []byte(base), o, th, LWW)
		b, err2 := Merge("schema.yaml", nil, []byte(base), th, o, LWW)
		if (err1 == nil) != (err2 == nil) || !bytes.Equal(a.Data, b.Data) || !sameOverwritten(a.Overwritten, b.Overwritten) || !slices.Equal(a.Notices, b.Notices) {
			t.Fatalf("not symmetric:\n%s\n---\n%s\n%v\n%v\n%v\n%v", a.Data, b.Data, a.Overwritten, b.Overwritten, a.Notices, b.Notices)
		}
	})
}
