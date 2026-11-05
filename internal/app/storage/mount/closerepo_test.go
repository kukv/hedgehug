package mount

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/storage/mount/mounttest"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
)

// newStoresIn creates the stores named names in one directory.
func newStoresIn(t *testing.T, root string, names ...string) []config.StoreEntry {
	t.Helper()
	var out []config.StoreEntry
	for i, name := range names {
		dir := filepath.Join(root, name)
		ws := &model.Workspace{ULID: model.NewULID(), Name: name, IDPrefix: string(rune('A'+i)) + "XX"}
		if _, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: name}, nil, ws, mounttest.Author, "", time.Now); err != nil {
			t.Fatal(err)
		}
		out = append(out, config.StoreEntry{Name: name, Path: dir})
	}
	return out
}

// After CloseRepo the store is not used any more, and a set opened again
// works; the closed one reopens its store when it is used (With).
func TestCloseRepoThenTheSetIsOpenedAgain(t *testing.T) {
	t.Parallel()
	e := mounttest.NewStoreAt(t, "here", "HRE")
	tasks := mounttest.PutTasks(t, e, 2, nil)
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
