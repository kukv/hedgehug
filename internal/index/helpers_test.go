package index

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

var testAuthor = gitrepo.Signature{Name: "Test", Email: "test@example.com"}

// newStore creates a store in a temporary directory and returns it with its
// directory.
func newStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "store")
	s, err := store.Create(dir, &model.StoreMeta{ID: model.NewID(), Name: "test"}, nil, testAuthor)
	if err != nil {
		t.Fatal(err)
	}
	return s, dir
}

func newTask(title string) *model.Task {
	return &model.Task{ID: model.NewID(), Kind: "task", Title: title, Status: "todo"}
}

// put writes tasks in one operation.
func put(t *testing.T, s *store.Store, tasks ...*model.Task) {
	t.Helper()
	if _, err := s.Update(store.Op{Name: "update", Author: testAuthor}, func(tx *store.Tx) error {
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

// commitRaw writes a file as given and commits it past the store's checks,
// as a sync or a hand edit could.
func commitRaw(t *testing.T, s *store.Store, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{rel}, Subject: "raw", Author: testAuthor, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
}

// rawTask is a task file with the given extra frontmatter lines.
func rawTask(id model.ID, lines string) string {
	return "---\nid: " + id.String() + "\n" + lines + "---\n"
}

func mustOpen(t *testing.T, s *store.Store) *Index {
	t.Helper()
	ix, err := Open(s)
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

func titles(es []*Entry) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.Title
	}
	return out
}
