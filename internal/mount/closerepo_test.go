package mount

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// newStoresIn creates the stores named names in one directory.
func newStoresIn(t *testing.T, root string, names ...string) []config.StoreEntry {
	t.Helper()
	var out []config.StoreEntry
	for i, name := range names {
		dir := filepath.Join(root, name)
		ws := &model.Workspace{ULID: model.NewULID(), Name: name, IDPrefix: string(rune('A'+i)) + "XX"}
		if _, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: name}, nil, ws, testAuthor, "", time.Now); err != nil {
			t.Fatal(err)
		}
		out = append(out, config.StoreEntry{Name: name, Path: dir})
	}
	return out
}

// Stores that share a directory read it once; each still gets its own
// leftovers, and none gets those of a neighbour.
func TestOpenStoresInOneDirectoryEachGetsItsOwnLeftover(t *testing.T) {
	es := newStoresIn(t, t.TempDir(), "one", "two", "three", "onetwo")
	// a rebuild beside "one", and an old store beside "two"; "three" and
	// "onetwo" (whose name starts like "one") have none
	if err := os.MkdirAll(es[0].Path+".rewrite-new", 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(es[1].Path+".before-rewrite-1", 0o750); err != nil {
		t.Fatal(err)
	}
	set := openSet(t, es...)
	if len(set.Stores) != 4 || len(set.Skipped) != 0 {
		t.Fatalf("stores %d, skipped %+v", len(set.Stores), set.Skipped)
	}
	var left []string
	for _, u := range set.Unfinished {
		var le *sync.LeftoverError
		if !errors.As(u.Err, &le) || le.Name != u.Name {
			t.Errorf("unfinished %+v", u)
		}
		left = append(left, u.Name)
	}
	if len(left) != 2 || left[0] != "one" || left[1] != "two" {
		t.Errorf("Unfinished = %v, want one and two", left)
	}
}

// After CloseRepo the store is not used any more, and a set opened again
// works; the closed one reopens its store when it is used (With).
func TestCloseRepoThenTheSetIsOpenedAgain(t *testing.T) {
	e := newStoreAt(t, "here", "HRE")
	tasks := putTasks(t, e, 2, nil)
	c := &config.Config{Stores: []config.StoreEntry{e}}
	set := Open(c, Options{})
	m, ok := set.Get("here")
	if !ok {
		t.Fatal("here is not mounted")
	}
	if _, err := m.Index(); err != nil {
		t.Fatal(err)
	}
	if err := m.CloseRepo(); err != nil {
		t.Fatal(err)
	}
	// the directory can be renamed now, and put back
	moved := e.Path + ".moved"
	if err := os.Rename(e.Path, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(moved, e.Path); err != nil {
		t.Fatal(err)
	}

	again := Open(c, Options{})
	m2, ok := again.Get("here")
	if !ok {
		t.Fatal("here is not mounted after opening again")
	}
	if err := m2.With(func(ix *index.Index) {
		if _, ok := ix.Task(tasks[1].ULID); !ok {
			t.Error("the task is not in the index of the set opened again")
		}
	}); err != nil {
		t.Fatal(err)
	}
	// the closed one opens its store again on use
	if err := m.With(func(ix *index.Index) {
		if _, ok := ix.Task(tasks[0].ULID); !ok {
			t.Error("the task is not in the index reopened by With")
		}
	}); err != nil {
		t.Fatalf("With after CloseRepo: %v", err)
	}
}
