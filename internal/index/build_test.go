package index

import (
	"bytes"
	"encoding/gob"
	"slices"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

// readCache decodes the cache file of s.
func readCache(t *testing.T, s *store.Store) cache {
	t.Helper()
	b, err := s.ReadLocal(cacheName)
	if err != nil {
		t.Fatal(err)
	}
	var c cache
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&c); err != nil {
		t.Fatal(err)
	}
	return c
}

func writeCache(t *testing.T, s *store.Store, c cache) {
	t.Helper()
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&c); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteLocal(cacheName, buf.Bytes()); err != nil {
		t.Fatal(err)
	}
}

func TestOpenWritesAndUsesCache(t *testing.T) {
	s, _ := newStore(t)
	task := newTask("original")
	put(t, s, task)
	ix := mustOpen(t, s)

	c := readCache(t, s)
	if c.Format != cacheFormat || c.Tree != ix.tree || len(c.Tasks) != 1 || c.Tasks[0].ID != task.ID {
		t.Fatalf("cache = %+v", c)
	}
	// Open trusts a cache for the same tree: an edited title shows that it
	// was read instead of the files.
	c.Tasks[0].Title = "from cache"
	writeCache(t, s, c)
	if e, _ := mustOpen(t, s).Task(task.ID); e.Title != "from cache" {
		t.Errorf("title = %q, want the cached one", e.Title)
	}
}

func TestOpenRebuildsBadCache(t *testing.T) {
	s, _ := newStore(t)
	task := newTask("original")
	put(t, s, task)
	ix := mustOpen(t, s)
	stale := readCache(t, s)
	stale.Tasks[0].Title = "stale"

	tests := []struct {
		name  string
		write func()
	}{
		{"garbage", func() {
			if err := s.WriteLocal(cacheName, []byte("garbage")); err != nil {
				t.Fatal(err)
			}
		}},
		{"other format", func() {
			c := stale
			c.Format = cacheFormat + 1
			writeCache(t, s, c)
		}},
		{"missing tree", func() {
			c := stale
			c.Tree = plumbing.NewHash("0123456789abcdef0123456789abcdef01234567")
			writeCache(t, s, c)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.write()
			e, ok := mustOpen(t, s).Task(task.ID)
			if !ok || e.Title != "original" {
				t.Errorf("entry = %+v, want the title from the files", e)
			}
			if c := readCache(t, s); c.Format != cacheFormat || c.Tree != ix.tree {
				t.Errorf("cache not rewritten: %+v", c)
			}
		})
	}
}

func TestRefreshReadsOnlyChanges(t *testing.T) {
	s, _ := newStore(t)
	kept, edited, purged := newTask("kept"), newTask("edited"), newTask("purged")
	put(t, s, kept, edited, purged)
	ix := mustOpen(t, s)
	keptBefore, _ := ix.Task(kept.ID)

	added := newTask("added")
	edited.Title = "edited again"
	put(t, s, added, edited)
	if _, err := s.Update(store.Op{Name: "purge", Author: testAuthor}, func(tx *store.Tx) error { return tx.RemoveTask(purged.ID) }); err != nil {
		t.Fatal(err)
	}
	if err := ix.Refresh(); err != nil {
		t.Fatal(err)
	}

	if e, ok := ix.Task(added.ID); !ok || e.Title != "added" {
		t.Errorf("added = %+v", e)
	}
	if e, _ := ix.Task(edited.ID); e.Title != "edited again" {
		t.Errorf("edited = %q", e.Title)
	}
	if _, ok := ix.Task(purged.ID); ok {
		t.Error("purged task still indexed")
	}
	if keptAfter, _ := ix.Task(kept.ID); keptAfter != keptBefore {
		t.Error("an unchanged entry was read again")
	}
	if c := readCache(t, s); c.Tree != ix.tree || len(c.Tasks) != 3 {
		t.Errorf("cache = %d tasks at %s, want 3 at %s", len(c.Tasks), c.Tree, ix.tree)
	}
	// Open from that cache gives the same tasks.
	got := titles(mustOpen(t, s).Tasks())
	slices.Sort(got)
	if want := []string{"added", "edited again", "kept"}; !slices.Equal(got, want) {
		t.Errorf("tasks after reopening = %v, want %v", got, want)
	}
}

func TestRefreshRevalidatesOnSchemaChange(t *testing.T) {
	s, dir := newStore(t)
	task := newTask("x")
	put(t, s, task)
	commitRaw(t, s, dir, model.TaskPath(task.ID), rawTask(task.ID, "key: TST-1\nworkspace: "+firstWorkspace(t, s).String()+"\nkind: task\ntitle: x\nstatus: todo\nprops:\n  priority: high\n"))
	ix := mustOpen(t, s)
	if e, _ := ix.Task(task.ID); len(e.Violations) != 0 {
		t.Fatalf("violations = %v", e.Violations)
	}

	sc := schema.Default()
	sc.Props[0].Values = []string{"low", "mid"} // priority without high
	if _, err := s.Update(store.Op{Name: "schema", Author: testAuthor}, func(tx *store.Tx) error { return tx.PutSchema(sc) }); err != nil {
		t.Fatal(err)
	}
	if err := ix.Refresh(); err != nil {
		t.Fatal(err)
	}
	if e, _ := ix.Task(task.ID); len(e.Violations) != 1 || e.Violations[0].Code != schema.CodeBadValue {
		t.Errorf("violations = %v, want a bad_value on priority", e.Violations)
	}
}

func TestRefreshRevalidatesChildren(t *testing.T) {
	s, _ := newStore(t)
	parent, child := newTask("parent"), newTask("child")
	parent.Kind, child.Kind = "epic", "story"
	child.Parent = &parent.ID
	put(t, s, parent, child)
	ix := mustOpen(t, s)

	parent.Kind = "task" // a story cannot sit under a task
	put(t, s, parent)
	if err := ix.Refresh(); err != nil {
		t.Fatal(err)
	}
	if e, _ := ix.Task(child.ID); len(e.Violations) != 1 || e.Violations[0].Code != schema.CodePlacement {
		t.Errorf("child violations = %v, want a placement violation", e.Violations)
	}
}

func TestRefreshWithoutChange(t *testing.T) {
	s, _ := newStore(t)
	put(t, s, newTask("x"))
	ix := mustOpen(t, s)
	if err := ix.Refresh(); err != nil {
		t.Errorf("Refresh = %v", err)
	}
}

func hasProblem(ix *Index, id model.ID, code string) bool {
	return slices.ContainsFunc(ix.Problems(id), func(p Problem) bool { return p.Code == code })
}

func TestProblemsIncludeViolationsOnOpen(t *testing.T) {
	s, dir := newStore(t)
	id := model.NewID()
	commitRaw(t, s, dir, model.TaskPath(id), rawTask(id, "kind: nope\ntitle: x\nstatus: todo\n"))
	if ix := mustOpen(t, s); !hasProblem(ix, id, schema.CodeUnknownKind) {
		t.Errorf("Problems after a rebuild = %v, want unknown_kind", ix.Problems(id))
	}
	if ix := mustOpen(t, s); !hasProblem(ix, id, schema.CodeUnknownKind) { // from the cache
		t.Errorf("Problems after a cached Open = %v, want unknown_kind", ix.Problems(id))
	}
}

func TestProblemsFollowRefresh(t *testing.T) {
	s, dir := newStore(t)
	task := newTask("x")
	put(t, s, task)
	commitRaw(t, s, dir, model.TaskPath(task.ID), rawTask(task.ID, "key: TST-1\nworkspace: "+firstWorkspace(t, s).String()+"\nkind: task\ntitle: x\nstatus: todo\nprops:\n  priority: high\n"))
	ix := mustOpen(t, s)
	if len(ix.Problems(task.ID)) != 0 {
		t.Fatalf("Problems = %v", ix.Problems(task.ID))
	}
	sc := schema.Default()
	sc.Props[0].Values = []string{"low", "mid"}
	if _, err := s.Update(store.Op{Name: "schema", Author: testAuthor}, func(tx *store.Tx) error { return tx.PutSchema(sc) }); err != nil {
		t.Fatal(err)
	}
	if err := ix.Refresh(); err != nil {
		t.Fatal(err)
	}
	if !hasProblem(ix, task.ID, schema.CodeBadValue) {
		t.Errorf("Problems after a schema change = %v, want bad_value", ix.Problems(task.ID))
	}
}

func TestProblemsFollowParentKind(t *testing.T) {
	s, _ := newStore(t)
	parent, child := newTask("parent"), newTask("child")
	parent.Kind, child.Kind = "epic", "story"
	child.Parent = &parent.ID
	put(t, s, parent, child)
	ix := mustOpen(t, s)
	parent.Kind = "task"
	put(t, s, parent)
	if err := ix.Refresh(); err != nil {
		t.Fatal(err)
	}
	if !hasProblem(ix, child.ID, schema.CodePlacement) {
		t.Errorf("child Problems = %v, want placement", ix.Problems(child.ID))
	}
}
