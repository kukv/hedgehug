package store

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

func TestCommitMerge(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	head, _ := s.repo.Head()
	files := map[string][]byte{"notes/a.md": []byte("a\n")}
	h, err := s.CommitMerge(head, []plumbing.Hash{head}, files, Op{Name: "repair", Detail: "1 issues", Author: testAuthor, Account: "github.com/a"}, []gitrepo.Trailer{{Key: "Hedgehug-Repaired", Value: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	c := headLog(t, s.dir)[0]
	if c.Hash != h || !strings.HasPrefix(c.Message, "hedgehug: repair 1 issues\n") || !strings.Contains(c.Message, "Hedgehug-Repaired: x") {
		t.Errorf("commit = %s %q", c.Hash, c.Message)
	}
	if !strings.Contains(c.Message, "Hedgehug-Op: repair\nHedgehug-Author: github.com/a\nHedgehug-Repaired: x") {
		t.Errorf("trailer order: %q", c.Message)
	}
	stale := map[string][]byte{"notes/stale.md": []byte("s\n")}
	if _, err := s.CommitMerge(head, []plumbing.Hash{head}, stale, Op{Name: "repair", Author: testAuthor}, nil); !errors.Is(err, ErrHeadMoved) {
		t.Errorf("stale head = %v", err)
	}
	if _, err := os.Stat(s.path("notes/stale.md")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a file was written on ErrHeadMoved: %v", err)
	}
}

func TestCommitMergeRefusesOtherFirstParent(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	head := mustHead(t, s)
	other := plumbing.NewHash("0123456789012345678901234567890123456789")
	files := map[string][]byte{"notes/a.md": []byte("a\n")}
	for _, parents := range [][]plumbing.Hash{nil, {other, head}} {
		if _, err := s.CommitMerge(head, parents, files, Op{Name: "merge", Author: testAuthor}, nil); err == nil || errors.Is(err, ErrHeadMoved) {
			t.Errorf("parents %v = %v", parents, err)
		}
	}
	if _, err := os.Stat(s.path("notes/a.md")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a file was written: %v", err)
	}
	if got := mustHead(t, s); got != head {
		t.Errorf("HEAD moved to %s", got)
	}
}

func TestCommitMergeWithUndecodableSchema(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	head := mustHead(t, s)
	old := s.Schema()
	h, err := s.CommitMerge(head, []plumbing.Hash{head}, map[string][]byte{schemaPath: []byte("{{ not yaml")}, Op{Name: "merge", Author: testAuthor}, nil)
	if err != nil || h.IsZero() {
		t.Fatalf("CommitMerge = %s, %v", h, err)
	}
	if got := mustHead(t, s); got != h {
		t.Errorf("HEAD = %s, want %s", got, h)
	}
	if s.Schema() != old {
		t.Error("the cached schema changed")
	}
}

func TestCommitMergeSkipsValidation(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	head, _ := s.repo.Head()
	x := model.NewULID()
	bad := []byte("---\nulid: " + x.String() + "\nid: TST-1\nworkspace: " + model.NewULID().String() + "\nkind: task\ntitle: x\nstatus: todo\n---\n")
	if _, err := s.CommitMerge(head, []plumbing.Hash{head}, map[string][]byte{model.TaskPath(x): bad}, Op{Name: "merge", Author: testAuthor}, nil); err != nil {
		t.Errorf("a merge with a broken invariant was refused: %v", err)
	}
}

func TestCommitMergeRollsBack(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	if _, err := s.Update(Op{Name: "edit", Author: testAuthor}, func(tx *Tx) error { return tx.PutFile("notes/old.md", []byte("old\n")) }); err != nil {
		t.Fatal(err)
	}
	head, _ := s.repo.Head()
	files := map[string][]byte{"notes/old.md": nil, "notes/new.md": []byte("n\n")}
	s.commit = func(gitrepo.Commit) (plumbing.Hash, error) { return plumbing.ZeroHash, errors.New("boom") }
	if _, err := s.CommitMerge(head, []plumbing.Hash{head}, files, Op{Name: "merge", Author: testAuthor}, nil); err == nil || errors.Is(err, gitrepo.ErrWorkTreeChanged) {
		t.Fatalf("a failed commit was not reported: %v", err)
	}
	if b, err := os.ReadFile(s.path("notes/old.md")); err != nil || string(b) != "old\n" {
		t.Errorf("old.md = %q, %v", b, err)
	}
	if _, err := os.Stat(s.path("notes/new.md")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("new.md stayed: %v", err)
	}
}

// A file saved by hand after the caller checked the work tree, and before
// the store lock, is not overwritten.
func TestCommitMergeWorkTreeChanged(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	head := mustHead(t, s)
	for name, rel := range map[string]string{"changed": schemaPath, "added": "notes/n.md"} {
		mustWrite(t, s, rel, "mine\n")
		_, err := s.CommitMerge(head, []plumbing.Hash{head}, map[string][]byte{rel: []byte("merged\n")}, Op{Name: "merge", Author: testAuthor}, nil)
		if !errors.Is(err, gitrepo.ErrWorkTreeChanged) {
			t.Errorf("%s: CommitMerge = %v, want ErrWorkTreeChanged", name, err)
		}
		if b, _ := os.ReadFile(s.path(rel)); string(b) != "mine\n" {
			t.Errorf("%s: %s = %q", name, rel, b)
		}
	}
	if got := mustHead(t, s); got != head {
		t.Errorf("HEAD moved to %s", got)
	}
}

func TestCommitMergeWithoutFiles(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	head := mustHead(t, s)
	// Both sides made the same change: the merge writes nothing, but its
	// second parent must still join the history.
	other := mustPut2(t, s)
	h, err := s.CommitMerge(other, []plumbing.Hash{other, head}, nil, Op{Name: "merge", Author: testAuthor}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c := headLog(t, s.dir)[0]; c.Hash != h || c.NumParents() != 2 {
		t.Errorf("commit = %s with %d parents", c.Hash, c.NumParents())
	}
	if _, err := s.CommitMerge(h, []plumbing.Hash{h}, nil, Op{Name: "merge", Author: testAuthor}, nil); !errors.Is(err, gitrepo.ErrNothingToCommit) {
		t.Errorf("one parent, no files = %v", err)
	}
}

// mustPut2 commits one more file and returns the new HEAD.
func mustPut2(t *testing.T, s *Store) plumbing.Hash {
	t.Helper()
	head := mustHead(t, s)
	h, err := s.CommitMerge(head, []plumbing.Hash{head}, map[string][]byte{"notes/b.md": []byte("b\n")}, Op{Name: "x", Author: testAuthor}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func mustHead(t *testing.T, s *Store) plumbing.Hash {
	t.Helper()
	h, err := s.repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// ahead makes a commit that changes schema.yaml and adds notes/n.md, then
// puts HEAD, the files and the cached schema back, and returns the old HEAD
// and the new commit.
func ahead(t *testing.T, s *Store) (before, to plumbing.Hash) {
	t.Helper()
	before = mustHead(t, s)
	old := s.Schema()
	if _, err := s.Update(Op{Name: "schema", Author: testAuthor}, func(tx *Tx) error { return tx.PutSchema(schema.Dev()) }); err != nil {
		t.Fatal(err)
	}
	head := mustHead(t, s)
	to, err := s.CommitMerge(head, []plumbing.Hash{head}, map[string][]byte{"notes/n.md": []byte("n\n")}, Op{Name: "x", Author: testAuthor}, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := git.PlainOpen(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	w, err := r.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Reset(&git.ResetOptions{Commit: before, Mode: git.HardReset}); err != nil {
		t.Fatal(err)
	}
	s.schema.Store(old)
	return before, to
}

func TestFastForward(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	before, to := ahead(t, s)
	dev, err := schema.Dev().Encode()
	if err != nil {
		t.Fatal(err)
	}
	if cur, _ := s.Schema().Encode(); string(cur) == string(dev) {
		t.Fatal("setup: the schema did not go back")
	}
	if err := s.FastForward(before, to, []string{"notes/n.md", "schema.yaml"}); err != nil {
		t.Fatal(err)
	}
	if got := mustHead(t, s); got != to {
		t.Errorf("HEAD = %s, want %s", got, to)
	}
	if b, _ := os.ReadFile(s.path("notes/n.md")); string(b) != "n\n" {
		t.Errorf("n.md = %q", b)
	}
	if b, _ := os.ReadFile(s.path("schema.yaml")); string(b) != string(dev) {
		t.Error("schema.yaml is not the new one")
	}
	if got, _ := s.Schema().Encode(); string(got) != string(dev) {
		t.Error("the cached schema was not refreshed")
	}
}

func TestFastForwardWorkTreeChanged(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	before, to := ahead(t, s)
	mustWrite(t, s, "notes/n.md", "mine\n")
	if err := s.FastForward(before, to, []string{"notes/n.md"}); !errors.Is(err, gitrepo.ErrWorkTreeChanged) {
		t.Errorf("FastForward = %v, want ErrWorkTreeChanged", err)
	}
	if got := mustHead(t, s); got != before {
		t.Errorf("HEAD moved to %s", got)
	}
}

func TestFastForwardChecksHead(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	head, _ := s.repo.Head()
	other := plumbing.NewHash("0123456789012345678901234567890123456789")
	if err := s.FastForward(other, head, nil); !errors.Is(err, ErrHeadMoved) {
		t.Errorf("stale head = %v", err)
	}
	if err := s.FastForward(head, other, nil); err == nil {
		t.Error("a missing commit was accepted")
	}
}
