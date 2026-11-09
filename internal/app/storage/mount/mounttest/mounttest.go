// Package mounttest makes stores for the tests of the packages that mount
// them (storage/mount and usecase).
package mounttest

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

const fixtureTitle = "fixture project"

// Author is the author of the commits the stores are made with.
var Author = gitrepo.Signature{Name: "Test", Email: "test@example.com"}

// NewStoreAt creates a store named name in a directory of its own, with a
// workspace of the id prefix prefix, and returns its entry.
func NewStoreAt(t *testing.T, name, prefix string) config.StoreEntry {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	ws := &model.Workspace{ULID: model.NewULID(), Name: name, IDPrefix: prefix}
	if _, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: name}, nil, ws, Author, "", time.Now); err != nil {
		t.Fatal(err)
	}
	return config.StoreEntry{Name: name, Path: dir}
}

// PutTasks writes n tasks into the store of e in one operation, edit
// changing each (i from 0) first, and returns them. The ids are assigned by
// the store, so the nth is PREFIX-n. A task that is not a project and has no
// parent goes under the fixture project, which the first call makes, in the
// same operation and after them, so that the tasks of that call keep the
// numbers 1 to n (the project is n+1). An id cannot be PREFIX-0.
func PutTasks(t *testing.T, e config.StoreEntry, n int, edit func(i int, task *model.Task)) []*model.Task {
	t.Helper()
	s, err := store.Open(e.Path)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := index.Open(s)
	if err != nil {
		t.Fatal(err)
	}
	ix.SetStoreName(e.Name)
	w := ix.Workspaces()[0]
	ws := w.ULID
	var tasks []*model.Task
	var fixture, made *model.Task
	for _, x := range ix.Tasks() {
		if x.Title == fixtureTitle {
			fixture = &model.Task{ULID: x.ULID}
		}
	}
	for i := range n {
		task := &model.Task{ULID: model.NewULID(), Title: e.Name, Kind: "task", Status: "todo", Workspace: ws}
		if edit != nil {
			edit(i, task)
		}
		if task.Parent == nil && task.Kind != "project" {
			if fixture == nil {
				fixture = &model.Task{ULID: model.NewULID(), Title: fixtureTitle, Kind: "project", Status: "todo", Workspace: ws}
				made = fixture
			}
			task.Parent = &fixture.ULID
		}
		tasks = append(tasks, task)
	}
	if _, err := s.Update(store.Op{Name: "add", Author: Author}, func(tx *store.Tx) error {
		for _, task := range tasks {
			if err := tx.PutTask(task); err != nil {
				return err
			}
		}
		if made != nil { // last, so that the tasks have the numbers 1 to n
			return tx.PutTask(made)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return tasks
}
