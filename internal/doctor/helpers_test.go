package doctor

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

var testAuthor = gitrepo.Signature{Name: "Test", Email: "test@example.com"}

// newStore creates a store with the default schema.
func newStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "store")
	s, err := store.Create(dir, &model.StoreMeta{ID: model.NewID(), Name: "test"}, schema.Default(), &model.Workspace{ID: model.NewID(), Name: "test", KeyPrefix: "TST"}, testAuthor)
	if err != nil {
		t.Fatal(err)
	}
	return s, dir
}

// rewriteSchema writes schema.yaml as the default schema after edit changed
// it, commits it past the checks of the store, and opens the store again.
func rewriteSchema(t *testing.T, dir string, edit func(*schema.Schema)) *store.Store {
	t.Helper()
	sc := schema.Default()
	edit(sc)
	b, err := sc.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, schema.Path), b, 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{schema.Path}, Subject: "raw", Author: testAuthor, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	return s
}

func newTask(title string) *model.Task {
	return &model.Task{ID: model.NewID(), Kind: "task", Title: title, Status: "todo"}
}

// put writes tasks in one operation, past the checks of transitions.
func put(t *testing.T, s *store.Store, tasks ...*model.Task) {
	t.Helper()
	if _, err := s.Update(store.Op{Name: "update", Author: testAuthor, SkipTransitions: true}, func(tx *store.Tx) error {
		for _, task := range tasks {
			if err := tx.PutTask(task); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// putRaw commits a task file past the checks of the store, as a sync or a
// hand edit could.
func putRaw(t *testing.T, s *store.Store, dir string, task *model.Task) {
	t.Helper()
	b, err := task.Encode()
	if err != nil {
		t.Fatal(err)
	}
	rel := model.TaskPath(task.ID)
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{rel}, Subject: "raw", Author: testAuthor, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
}

func mustOpen(t *testing.T, s *store.Store) *index.Index {
	t.Helper()
	ix, err := index.Open(s)
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

func checks(fs []Finding) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Check
	}
	return out
}

func commitOf(paths ...string) gitrepo.Commit {
	return gitrepo.Commit{Paths: paths, Subject: "raw", Author: testAuthor, When: time.Now()}
}
