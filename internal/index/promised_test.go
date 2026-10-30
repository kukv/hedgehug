package index

import (
	"cmp"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

func byULID(es []*Entry) []*Entry {
	out := slices.Clone(es)
	slices.SortFunc(out, func(a, b *Entry) int { return cmp.Compare(a.ULID.String(), b.ULID.String()) })
	return out
}

func TestOpenParentCycleOfThree(t *testing.T) {
	s, dir := newStore(t)
	a, b, c := model.NewULID(), model.NewULID(), model.NewULID()
	for id, parent := range map[model.ULID]model.ULID{a: b, b: c, c: a} {
		commitRaw(t, s, dir, model.TaskPath(id), rawTask(id, "kind: task\ntitle: x\nstatus: todo\nparent: "+parent.String()+"\n"))
	}
	loose := newTask("loose")
	put(t, s, loose)

	ix, err := Open(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []model.ULID{a, b, c} {
		if !hasProblem(ix, id, "cycle") {
			t.Errorf("task %s has no cycle problem: %v", id, ix.Problems(id))
		}
	}
	if hasProblem(ix, loose.ULID, "cycle") {
		t.Error("a task outside the cycle has a cycle problem")
	}
	roots := ix.Roots()
	if len(roots) != 1 || roots[0].ULID != loose.ULID {
		t.Errorf("roots = %v, want only the task outside the cycle", titles(roots))
	}
	// Again from the cache, and after a Refresh.
	ix2 := mustOpen(t, s)
	if !hasProblem(ix2, a, "cycle") || len(ix2.Roots()) != 1 {
		t.Error("the cycle is not found when opened from the cache")
	}
	put(t, s, newTask("later"))
	if err := ix2.Refresh(); err != nil {
		t.Fatal(err)
	}
	if !hasProblem(ix2, b, "cycle") || len(ix2.Roots()) != 2 {
		t.Error("the cycle is not found after Refresh")
	}
}

func TestOpenCacheEqualsRebuild(t *testing.T) {
	s, dir := newStore(t)
	p := &model.Task{ULID: model.NewULID(), Kind: "project", Title: "P", Status: "todo"}
	child := newTask("child")
	child.Parent = &p.ULID
	child.Labels = []string{"x", "y"}
	other := newTask("other")
	other.BlockedBy = []model.ULID{child.ULID}
	gone := newTask("gone")
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	gone.DeletedAt = &now
	put(t, s, p, child, other, gone)
	commitRaw(t, s, dir, model.TaskPath(model.NewULID()), "not a task file\n")

	mustOpen(t, s) // writes the cache
	cached := mustOpen(t, s)
	if err := os.Remove(filepath.Join(dir, ".hedgehug", "local", cacheName)); err != nil {
		t.Fatal(err)
	}
	fresh := mustOpen(t, s)

	if len(fresh.Tasks()) != 5 {
		t.Errorf("tasks = %d, want 5", len(fresh.Tasks()))
	}
	if a, b := byULID(cached.Tasks()), byULID(fresh.Tasks()); !reflect.DeepEqual(a, b) {
		t.Errorf("entries differ between the cache and a rebuild:\ncache:   %+v\nrebuild: %+v", a, b)
	}
}

func TestOpenAfterFailedUpdate(t *testing.T) {
	s, dir := newStore(t)
	put(t, s, newTask("kept"))
	ix := mustOpen(t, s)

	// A schema.yaml that does not decode makes update fail.
	put(t, s, newTask("added"))
	commitRaw(t, s, dir, schema.Path, "kinds: [: broken\n")
	if err := ix.Refresh(); err == nil {
		t.Fatal("Refresh succeeded with a broken schema.yaml")
	}
	if _, err := Open(s); err == nil {
		t.Fatal("Open succeeded with a broken schema.yaml")
	}

	good, err := schema.Default().Encode()
	if err != nil {
		t.Fatal(err)
	}
	commitRaw(t, s, dir, schema.Path, string(good))
	ix2 := mustOpen(t, s)
	got := titles(ix2.Tasks())
	slices.Sort(got)
	if want := []string{"added", "kept"}; !slices.Equal(got, want) {
		t.Errorf("tasks = %v, want %v", got, want)
	}
	if c := readCache(t, s); c.Tree != ix2.tree || len(c.Tasks) != 2 {
		t.Errorf("cache = %d tasks at %s, want 2 at %s", len(c.Tasks), c.Tree, ix2.tree)
	}
}
