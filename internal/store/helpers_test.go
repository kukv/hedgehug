package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
)

var (
	testNow    = time.Date(2026, 10, 30, 12, 0, 0, 0, time.FixedZone("", 9*60*60))
	testAuthor = gitrepo.Signature{Name: "Test", Email: "test@example.com"}

	// testWorkspaceULID is the ULID of the workspace of every store newTestStore
	// makes, so newTask can name it without a store at hand.
	testWorkspaceULID = model.NewULID()
)

// newTestStore creates a store in a temporary directory, with a fixed clock.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	meta := &model.StoreMeta{ULID: model.NewULID(), Name: "test"}
	s, err := Create(filepath.Join(t.TempDir(), "store"), meta, nil, &model.Workspace{ULID: testWorkspaceULID, Name: "test", IDPrefix: "TST"}, testAuthor, "", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return testNow }
	return s
}

// testWorkspace returns the workspace newTestStore made.
func testWorkspace(t *testing.T, s *Store) *model.Workspace {
	t.Helper()
	for w, err := range s.Workspaces() {
		if err != nil {
			t.Fatal(err)
		}
		return w
	}
	t.Fatal("the store has no workspace")
	return nil
}

// headLog returns the commits reachable from HEAD, newest first.
func headLog(t *testing.T, dir string) []*object.Commit {
	t.Helper()
	r, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	it, err := r.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		t.Fatal(err)
	}
	var out []*object.Commit
	if err := it.ForEach(func(c *object.Commit) error {
		out = append(out, c)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

// mustWrite writes a file into the store directly, bypassing Update.
func mustWrite(t *testing.T, s *Store, rel, content string) {
	t.Helper()
	p := s.path(rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// mustPut writes tasks in one operation and fails the test if it fails.
func mustPut(t *testing.T, s *Store, tasks ...*model.Task) {
	t.Helper()
	if err := put(s, tasks...); err != nil {
		t.Fatal(err)
	}
}
