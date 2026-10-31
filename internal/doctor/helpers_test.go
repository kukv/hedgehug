package doctor

import (
	"fmt"
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
	s, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "test"}, schema.Default(), &model.Workspace{ULID: model.NewULID(), Name: "test", IDPrefix: "TST"}, testAuthor, "", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Repo().RegisterMergeDriver(); err != nil {
		t.Fatal(err)
	}
	return s, dir
}

// newBareStore creates a store whose .git/config has no merge driver.
func newBareStore(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "store")
	if _, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "test"}, schema.Default(), &model.Workspace{ULID: model.NewULID(), Name: "test", IDPrefix: "TST"}, testAuthor, "", time.Now); err != nil {
		t.Fatal(err)
	}
	return dir
}

func init() {
	// hedgehug is on PATH for every test but the ones that say otherwise.
	lookPath = func(string) (string, error) { return "/bin/hedgehug", nil }
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
	return &model.Task{ULID: model.NewULID(), Kind: "task", Title: title, Status: "todo"}
}

// put writes tasks in one operation, past the checks of transitions.
func put(t *testing.T, s *store.Store, tasks ...*model.Task) {
	t.Helper()
	ws := firstWorkspace(t, s)
	if _, err := s.Update(store.Op{Name: "update", Author: testAuthor, SkipTransitions: true}, func(tx *store.Tx) error {
		for _, task := range tasks {
			if task.Workspace.IsZero() {
				task.Workspace = ws
			}
			if err := tx.PutTask(task); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// firstWorkspace returns the ULID of the first workspace of s.
func firstWorkspace(t *testing.T, s *store.Store) model.ULID {
	t.Helper()
	for w, err := range s.Workspaces() {
		if err != nil {
			t.Fatal(err)
		}
		return w.ULID
	}
	t.Fatal("the store has no workspace")
	return model.ULID{}
}

// rawIDs numbers the ids putRaw gives to tasks without one.
var rawIDs int

// putRaw commits a task file past the checks of the store, as a sync or a
// hand edit could.
func putRaw(t *testing.T, s *store.Store, dir string, task *model.Task) {
	t.Helper()
	if task.Workspace.IsZero() {
		task.Workspace = firstWorkspace(t, s)
	}
	if task.ID == "" {
		rawIDs++
		task.ID = fmt.Sprintf("TST-%d", 1000+rawIDs)
	}
	b, err := task.Encode()
	if err != nil {
		t.Fatal(err)
	}
	rel := model.TaskPath(task.ULID)
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
