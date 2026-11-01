package mount

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

var testAuthor = gitrepo.Signature{Name: "Test", Email: "test@example.com"}

// newStoreAt creates a store named name in a directory of its own, with a
// workspace of the id prefix prefix, and returns its entry.
func newStoreAt(t *testing.T, name, prefix string) config.StoreEntry {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	ws := &model.Workspace{ULID: model.NewULID(), Name: name, IDPrefix: prefix}
	if _, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: name}, nil, ws, testAuthor, "", time.Now); err != nil {
		t.Fatal(err)
	}
	return config.StoreEntry{Name: name, Path: dir}
}

// putTasks writes n tasks into the store of e in one operation, edit
// changing each (i from 0) first, and returns them. The ids are assigned by
// the store, so the nth is PREFIX-n.
func putTasks(t *testing.T, e config.StoreEntry, n int, edit func(i int, task *model.Task)) []*model.Task {
	t.Helper()
	s, err := store.Open(e.Path)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := (&Mounted{Name: e.Name, Entry: e, Store: s}).Index()
	if err != nil {
		t.Fatal(err)
	}
	ws := ix.Workspaces()[0].ULID
	var tasks []*model.Task
	for i := range n {
		task := &model.Task{ULID: model.NewULID(), Title: e.Name, Kind: "task", Status: "todo", Workspace: ws}
		if edit != nil {
			edit(i, task)
		}
		tasks = append(tasks, task)
	}
	if _, err := s.Update(store.Op{Name: "add", Author: testAuthor}, func(tx *store.Tx) error {
		for _, task := range tasks {
			if err := tx.PutTask(task); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return tasks
}

// openSet opens the stores of es.
func openSet(t *testing.T, es ...config.StoreEntry) *Set {
	t.Helper()
	set, err := Open(&config.Config{Stores: es}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return set
}
