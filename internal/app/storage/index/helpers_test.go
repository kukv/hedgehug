package index

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

var testAuthor = gitrepo.Signature{Name: "Test", Email: "test@example.com"}

// newStore creates a store in a temporary directory and returns it with its
// directory.
func newStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "store")
	sc := schema.Default()
	sc.Labels = []string{"backend", "x", "y"}
	s, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "test"}, sc, &model.Workspace{ULID: model.NewULID(), Name: "test", IDPrefix: "TST"}, testAuthor, "", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	return s, dir
}

func newTask(title string) *model.Task {
	return &model.Task{ULID: model.NewULID(), Kind: "project", Title: title, Status: "todo"}
}

// put writes tasks in one operation.
func put(t *testing.T, s *store.Store, tasks ...*model.Task) {
	t.Helper()
	ws := firstWorkspace(t, s)
	if _, err := s.Update(store.Op{Name: "update", Author: testAuthor}, func(tx *store.Tx) error {
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

// renumber gives task's workspace the id prefix p and writes task again,
// which gives it an id of p and keeps the old one among its former ids.
func renumber(t *testing.T, s *store.Store, task *model.Task, p string) {
	t.Helper()
	if _, err := s.Update(store.Op{Name: "workspace", Author: testAuthor}, func(tx *store.Tx) error {
		w, err := tx.Workspace(task.Workspace)
		if err != nil {
			return err
		}
		w.IDPrefix = p
		return errors.Join(tx.PutWorkspace(w), tx.PutTask(task))
	}); err != nil {
		t.Fatal(err)
	}
}

// putComment writes the comments cs on task in one operation.
func putComment(t *testing.T, s *store.Store, task model.ULID, cs ...*model.Comment) {
	t.Helper()
	if _, err := s.Update(store.Op{Name: "comment", Task: task, Author: testAuthor}, func(tx *store.Tx) error {
		for _, c := range cs {
			if err := tx.PutComment(task, c); err != nil {
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
func rawTask(ulid model.ULID, lines string) string {
	return "---\nulid: " + ulid.String() + "\n" + lines + "---\n"
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

// hideObjectAt moves the loose object of the file or directory at rel in
// HEAD of the repository in dir out of the object database, so that a read
// of it fails with plumbing.ErrObjectNotFound. restore puts it back; the
// cleanup of the test does too. The test fails when the object is not loose:
// the objects of a test store are, until something packs them.
func hideObjectAt(t *testing.T, dir, rel string) (restore func()) {
	t.Helper()
	r, err := gogit.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	head, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	c, err := r.CommitObject(head.Hash())
	if err != nil {
		t.Fatal(err)
	}
	tree, err := c.Tree()
	if err != nil {
		t.Fatal(err)
	}
	e, err := tree.FindEntry(rel)
	if err != nil {
		t.Fatal(err)
	}
	h := e.Hash.String()
	loose := filepath.Join(dir, ".git", "objects", h[:2], h[2:])
	hidden := loose + ".hidden"
	if err := os.Rename(loose, hidden); err != nil {
		t.Fatalf("%s is not a loose object: %v", rel, err)
	}
	restore = func() {
		if _, err := os.Stat(hidden); err == nil {
			if err := os.Rename(hidden, loose); err != nil {
				t.Error(err)
			}
		}
	}
	t.Cleanup(restore)
	return restore
}
