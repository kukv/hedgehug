package merge

import (
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
)

// priority, start, due, estimate and points are scalars: the later side wins
// and the value that loses is told.
func TestBuiltinPropertiesAreScalars(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ key, base, ours, theirs string }{
		{"priority", "low", "mid", "high"},
		{"start", "2026-11-01", "2026-11-02", "2026-11-03"},
		{"due", "2026-11-10", "2026-11-11", "2026-11-12"},
		{"estimate", "1h", "2h", "3h"},
		{"points", "1", "2", "3"},
	} {
		base := task("2026-10-31T10:00:00Z", c.key+": "+c.base)
		ours := task("2026-10-31T11:00:00Z", c.key+": "+c.ours)
		theirs := task("2026-10-31T12:00:00Z", c.key+": "+c.theirs)
		r := mergeTask(t, side(base, ""), side(ours, "github.com/a"), side(theirs, "github.com/b"))
		want := task("2026-10-31T12:00:00Z", c.key+": "+c.theirs)
		if string(r.Data) != string(want) {
			t.Errorf("%s: merged =\n%s", c.key, r.Data)
		}
		if len(r.Overwritten) != 1 || r.Overwritten[0].Field != c.key || r.Overwritten[0].Author != "github.com/a" {
			t.Errorf("%s: overwritten = %v", c.key, r.Overwritten)
		}
	}
}

func TestBuiltinPropertyOneSide(t *testing.T) {
	t.Parallel()
	base := task("2026-10-31T10:00:00Z", "title: a", "due: 2026-11-10")
	ours := task("2026-10-31T11:00:00Z", "title: b", "due: 2026-11-10")
	theirs := task("2026-10-31T12:00:00Z", "title: a", "due: 2026-11-12", "points: 0")
	r := mergeTask(t, side(base, ""), side(ours, "github.com/a"), side(theirs, "github.com/b"))
	x := decoded(t, r.Data)
	if x.Title != "b" || x.Due.String() != "2026-11-12" || x.Points == nil || *x.Points != 0 || len(r.Overwritten) != 0 {
		t.Errorf("task = %+v, overwritten = %v", x, r.Overwritten)
	}
}

func TestTemplateBuiltinPropertiesAreScalars(t *testing.T) {
	t.Parallel()
	p := "templates/t.md"
	tmpl := func(updated string, lines ...string) []byte {
		return []byte("---\n" + strings.Join(lines, "\n") + "\nupdated: " + updated + "\n---\n")
	}
	base := tmpl("2026-10-31T10:00:00Z", "priority: low", "due: 2026-11-10", "points: 1")
	ours := tmpl("2026-10-31T11:00:00Z", "priority: high", "due: 2026-11-11", "points: 1")
	theirs := tmpl("2026-10-31T12:00:00Z", "priority: low", "due: 2026-11-12", "points: 2")
	r := mergeSym(t, p, side(base, ""), side(ours, "github.com/a"), side(theirs, "github.com/b"))
	x, err := model.DecodeTemplate("t", r.Data)
	if err != nil {
		t.Fatal(err)
	}
	if it := x.Item; it.Priority != "high" || it.Due.String() != "2026-11-12" || *it.Points != 2 {
		t.Errorf("item = %+v\n%s", it, r.Data)
	}
	want := []Overwritten{{Path: p, Field: "due", Value: `"2026-11-11T00:00:00Z"`, Author: "github.com/a"}}
	if !sameOverwritten(r.Overwritten, want) {
		t.Errorf("overwritten = %v", r.Overwritten)
	}
}

// labels of schema.yaml merge as a set against the base: what either side
// added or removed is kept. priorities stay whole, the later side's.
func TestSchemaLabelsMergeAsASet(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, base, ours, theirs, want string }{
		{"both add", "labels: [bug]\n", "labels: [bug, docs]\n", "labels: [bug, infra]\n", "labels: [bug, docs, infra]\n"},
		{"one adds, one removes", "labels: [bug, docs]\n", "labels: [bug, docs, infra]\n", "labels: [docs]\n", "labels: [docs, infra]\n"},
		{"both remove different", "labels: [a, b, c]\n", "labels: [a, b]\n", "labels: [b, c]\n", "labels: [b]\n"},
		{"added from nothing", "statuses: []\n", "statuses: []\nlabels: [a]\n", "statuses: []\nlabels: [b]\n", "statuses: []\nlabels: [a, b]\n"},
		{"same add", "labels: [a]\n", "labels: [a, b]\n", "labels: [a, b]\n", "labels: [a, b]\n"},
	} {
		r := mergeSym(t, schema.Path, Side{Data: []byte(c.base)}, Side{Data: []byte(c.ours), Author: "a", When: whenAt(time.Unix(1, 0))}, Side{Data: []byte(c.theirs), Author: "b", When: whenAt(time.Unix(2, 0))})
		if string(r.Data) != c.want || len(r.Overwritten) != 0 {
			t.Errorf("%s: merged = %q, overwritten = %v, want %q", c.name, r.Data, r.Overwritten, c.want)
		}
	}
}

func TestSchemaPrioritiesMergeWhole(t *testing.T) {
	t.Parallel()
	base := []byte("priorities: [low, high]\n")
	ours := []byte("priorities: [low, mid, high]\n")
	theirs := []byte("priorities: [low, high, urgent]\n")
	r := mergeSym(t, schema.Path, Side{Data: base}, Side{Data: ours, Author: "a", When: whenAt(time.Unix(1, 0))}, Side{Data: theirs, Author: "b", When: whenAt(time.Unix(2, 0))})
	if string(r.Data) != string(theirs) {
		t.Errorf("merged = %q, want the later side's list", r.Data)
	}
	if len(r.Overwritten) != 1 || r.Overwritten[0].Field != "priorities" || r.Overwritten[0].Author != "a" {
		t.Errorf("overwritten = %v", r.Overwritten)
	}
}
