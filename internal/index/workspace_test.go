package index

import (
	"errors"
	"sync"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

func TestWorkspacesAndResolveWorkspace(t *testing.T) {
	s, _ := newStore(t)
	api := &model.Workspace{ID: model.NewID(), Name: "api", KeyPrefix: "API"}
	if _, err := s.Update(store.Op{Name: "workspace", Author: testAuthor}, func(tx *store.Tx) error { return tx.PutWorkspace(api) }); err != nil {
		t.Fatal(err)
	}
	ix := mustOpen(t, s)
	if ws := ix.Workspaces(); len(ws) != 2 || ws[0].Name != "api" || ws[1].Name != "test" {
		t.Fatalf("Workspaces = %+v", ws)
	}
	for _, in := range []string{"api", "API", "Api", api.ID.String()} {
		if w, err := ix.ResolveWorkspace(in); err != nil || w.ID != api.ID {
			t.Errorf("ResolveWorkspace(%q) = %v %v", in, w, err)
		}
	}
	if _, err := ix.ResolveWorkspace("nope"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("nope: %v", err)
	}
	x := newTask("x")
	x.Workspace = api.ID
	put(t, s, x)
	if err := ix.Refresh(); err != nil {
		t.Fatal(err)
	}
	if e, _ := ix.Task(x.ID); e.Workspace != api.ID || e.Key != "API-1" {
		t.Errorf("entry = %+v", e)
	}
	if n, err := ix.KeyNumbers(); err != nil || n["API"] != 1 || n["TST"] != 0 {
		t.Errorf("KeyNumbers = %v %v", n, err)
	}
}

func TestResolveWorkspaceAmbiguous(t *testing.T) {
	s, _ := newStore(t)
	// The name of one workspace is the prefix of another.
	other := &model.Workspace{ID: model.NewID(), Name: "tst", KeyPrefix: "OTH"}
	if _, err := s.Update(store.Op{Name: "workspace", Author: testAuthor}, func(tx *store.Tx) error { return tx.PutWorkspace(other) }); err != nil {
		t.Fatal(err)
	}
	ix := mustOpen(t, s)
	var amb *AmbiguousWorkspaceError
	// "tst" is the name of one workspace and the prefix of the other.
	if _, err := ix.ResolveWorkspace("tst"); !errors.As(err, &amb) || len(amb.Candidates) != 2 {
		t.Errorf("ResolveWorkspace(tst) err = %v, want two candidates", err)
	}
}

func TestWorkspacesAfterRefresh(t *testing.T) {
	s, _ := newStore(t)
	ix := mustOpen(t, s)
	api := &model.Workspace{ID: model.NewID(), Name: "api", KeyPrefix: "API"}
	if _, err := s.Update(store.Op{Name: "workspace", Author: testAuthor}, func(tx *store.Tx) error { return tx.PutWorkspace(api) }); err != nil {
		t.Fatal(err)
	}
	if err := ix.Refresh(); err != nil {
		t.Fatal(err)
	}
	if _, ok := ix.Workspace(api.ID); !ok {
		t.Error("Refresh did not read the new workspace")
	}
	if ix2 := mustOpen(t, s); len(ix2.Workspaces()) != 2 {
		t.Errorf("Open from the cache: Workspaces = %+v", ix2.Workspaces())
	}
}

func TestKeyNumbersIsAFreshMap(t *testing.T) {
	s, _ := newStore(t)
	put(t, s, newTask("a"))
	ix := mustOpen(t, s)
	n, _ := ix.KeyNumbers()
	n["TST"] = 99
	if n2, _ := ix.KeyNumbers(); n2["TST"] != 1 {
		t.Errorf("KeyNumbers shares its map: %v", n2)
	}
}

// Two handles of one store, each with an index, add tasks at once: the keys
// stay distinct because Update counts with the index under the store's lock.
func TestConcurrentAddsThroughIndexes(t *testing.T) {
	s1, dir := newStore(t)
	s2, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	mustOpen(t, s1)
	mustOpen(t, s2)
	ws := firstWorkspace(t, s1)
	const per = 8
	var wg sync.WaitGroup
	for _, s := range []*store.Store{s1, s2} {
		for range per {
			wg.Add(1)
			go func() {
				defer wg.Done()
				task := newTask("x")
				task.Workspace = ws
				_, err := s.Update(store.Op{Name: "add", Author: testAuthor}, func(tx *store.Tx) error { return tx.PutTask(task) })
				if err != nil {
					t.Error(err)
				}
			}()
		}
	}
	wg.Wait()
	keys := map[string]bool{}
	for task, err := range s1.Tasks() {
		if err != nil {
			t.Fatal(err)
		}
		if keys[task.Key] {
			t.Errorf("key %s is given twice", task.Key)
		}
		keys[task.Key] = true
	}
	if len(keys) != 2*per {
		t.Errorf("%d keys, want %d", len(keys), 2*per)
	}
}
