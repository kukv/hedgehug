package gitrepo

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
)

// rewritten is a repository at from with x.md and old.md, and a commit to
// that is not a descendant of it and has x.md changed, new.md added and
// old.md gone.
func rewritten(t *testing.T) (r *Repo, dir string, from, to plumbing.Hash, paths []string) {
	t.Helper()
	remote := newBare(t)
	a, _ := cloneTwo(t, remote)
	root, err := a.r.Head()
	if err != nil {
		t.Fatal(err)
	}
	from = commitHead(t, a.r, a.dir, map[string]string{"x.md": "old secret\n", "old.md": "gone\n"})
	to, err = a.r.CommitOn(root, map[string][]byte{"x.md": []byte("new\n"), "new.md": []byte("added\n")}, Commit{
		Subject: "rewritten", Author: Signature{Name: "a", Email: "a@example.com"}, When: time.Unix(1, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	return a.r, a.dir, from, to, []string{"new.md", "old.md", "x.md"}
}

func TestCommitOnLeavesHeadAndTheWorkTree(t *testing.T) {
	t.Parallel()
	r, dir, from, to, _ := rewritten(t)
	if h, err := r.Head(); err != nil || h != from {
		t.Errorf("Head = %s, %v; want %s", h, err, from)
	}
	if _, err := os.Stat(filepath.Join(dir, "new.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("new.md on disk: %v", err)
	}
	if st, err := r.Status(); err != nil || len(st) != 0 {
		t.Errorf("Status = %v, %v", st, err)
	}
	if b, err := r.ReadAt(to, "x.md"); err != nil || string(b) != "new\n" {
		t.Errorf("x.md at to = %q, %v", b, err)
	}
}

func TestCommitOnRefusesAPathOutsideTheWorkTree(t *testing.T) {
	t.Parallel()
	r, _, from, _, _ := rewritten(t)
	if _, err := r.CommitOn(from, map[string][]byte{".git/config": []byte("x")}, Commit{Subject: "s"}); err == nil {
		t.Error("CommitOn wrote into .git")
	}
}

func checkMoved(t *testing.T, r *Repo, dir string, to plumbing.Hash) {
	t.Helper()
	if h, err := r.Head(); err != nil || h != to {
		t.Errorf("Head = %s, %v; want %s", h, err, to)
	}
	if st, err := r.Status(); err != nil || len(st) != 0 {
		t.Errorf("Status = %v, %v", st, err)
	}
	for p, want := range map[string]string{"x.md": "new\n", "new.md": "added\n"} {
		if b, err := os.ReadFile(filepath.Join(dir, p)); err != nil || string(b) != want {
			t.Errorf("%s = %q, %v", p, b, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "old.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("old.md: %v", err)
	}
}

func TestMoveMainToACommitThatIsNoDescendant(t *testing.T) {
	t.Parallel()
	r, dir, from, to, paths := rewritten(t)
	if err := r.MoveMain(from, to, paths); err != nil {
		t.Fatal(err)
	}
	checkMoved(t, r, dir, to)
}

// A run that stopped part of the way is finished by running it again: some
// paths are written, or everything and main moved.
func TestMoveMainFinishesARunThatStopped(t *testing.T) {
	t.Parallel()
	r, dir, from, to, paths := rewritten(t)
	if err := os.WriteFile(filepath.Join(dir, "x.md"), []byte("new\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "old.md")); err != nil {
		t.Fatal(err)
	}
	if err := r.MoveMain(from, to, paths); err != nil {
		t.Fatal(err)
	}
	checkMoved(t, r, dir, to)
	if err := r.MoveMain(from, to, paths); err != nil {
		t.Fatalf("again: %v", err)
	}
	checkMoved(t, r, dir, to)
}

func TestMoveMainLeavesAFileEditedByHand(t *testing.T) {
	t.Parallel()
	r, dir, from, to, paths := rewritten(t)
	p := filepath.Join(dir, "x.md")
	if err := os.WriteFile(p, []byte("by hand\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := r.MoveMain(from, to, paths); !errors.Is(err, ErrWorkTreeChanged) {
		t.Fatalf("MoveMain = %v, want ErrWorkTreeChanged", err)
	}
	if b, _ := os.ReadFile(p); string(b) != "by hand\n" {
		t.Errorf("x.md = %q", b)
	}
	if h, err := r.Head(); err != nil || h != from {
		t.Errorf("Head = %s, %v; want %s", h, err, from)
	}
}

func TestMoveMainRefusesAHeadOfNeitherHistory(t *testing.T) {
	t.Parallel()
	r, dir, from, to, paths := rewritten(t)
	commitHead(t, r, dir, map[string]string{"z.md": "z\n"})
	if err := r.MoveMain(from, to, paths); !errors.Is(err, ErrWorkTreeChanged) {
		t.Fatalf("MoveMain = %v, want ErrWorkTreeChanged", err)
	}
}

func TestPruneUnreachableRemovesTheOldHistory(t *testing.T) {
	t.Parallel()
	r, dir, from, to, paths := rewritten(t)
	if err := r.Consolidate(); err != nil { // the old history in a pack too
		t.Fatal(err)
	}
	if err := r.MoveMain(from, to, paths); err != nil {
		t.Fatal(err)
	}
	if err := r.SetTracking("origin", to); err != nil {
		t.Fatal(err)
	}
	if err := r.PruneUnreachable(); err != nil {
		t.Fatal(err)
	}
	// Opened again: a repository keeps the objects it read in memory.
	fresh, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fresh.Close() }()
	if err := fresh.Verify(); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if _, err := fresh.TreeOf(from); !errors.Is(err, plumbing.ErrObjectNotFound) {
		t.Errorf("the old head reads: %v", err)
	}
	if n, err := fresh.LooseObjects(); err != nil || n != 0 {
		t.Errorf("loose = %d, %v", n, err)
	}
	if got := len(packFiles(t, dir)); got != 2 {
		t.Errorf("pack files = %v, want one pack and its index", packFiles(t, dir))
	}
	if b, err := fresh.ReadAt(to, "new.md"); err != nil || string(b) != "added\n" {
		t.Errorf("new.md = %q, %v", b, err)
	}
	// A blob is compressed on disk, so the hash stands for its bytes.
	gone := plumbing.ComputeHash(plumbing.BlobObject, []byte("old secret\n"))
	if ok, err := fresh.hasObject(gone); err != nil || ok {
		t.Errorf("the old blob: %v, %v", ok, err)
	}
	// A run that stopped is finished by the next.
	if err := r.PruneUnreachable(); err != nil {
		t.Fatalf("again: %v", err)
	}
}

// hasObject reports whether the object h is stored.
func (r *Repo) hasObject(h plumbing.Hash) (bool, error) {
	err := r.r.Storer.HasEncodedObject(h)
	if errors.Is(err, plumbing.ErrObjectNotFound) {
		return false, nil
	}
	return err == nil, err
}

func TestPruneUnreachableKeepsWhatTheTrackingRefReaches(t *testing.T) {
	t.Parallel()
	r, _, from, to, paths := rewritten(t)
	if err := r.MoveMain(from, to, paths); err != nil {
		t.Fatal(err)
	}
	if err := r.SetTracking("origin", from); err != nil {
		t.Fatal(err)
	}
	if err := r.PruneUnreachable(); err != nil {
		t.Fatal(err)
	}
	if b, err := r.ReadAt(from, "x.md"); err != nil || !bytes.Equal(b, []byte("old secret\n")) {
		t.Errorf("x.md at from = %q, %v", b, err)
	}
}

func TestKeepMainRefs(t *testing.T) {
	t.Parallel()
	r, _, from, _, _ := rewritten(t)
	others := []plumbing.ReferenceName{"refs/remotes/mirror/main", "refs/tags/v1", "refs/heads/other"}
	for _, n := range others {
		if err := r.r.Storer.SetReference(plumbing.NewHashReference(n, from)); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.KeepMainRefs(); err != nil {
		t.Fatal(err)
	}
	for _, n := range others {
		if _, err := r.r.Storer.Reference(n); !errors.Is(err, plumbing.ErrReferenceNotFound) {
			t.Errorf("%s: %v", n, err)
		}
	}
	for _, n := range []plumbing.ReferenceName{plumbing.HEAD, mainRef, trackingRef("origin")} {
		if _, err := r.r.Storer.Reference(n); err != nil {
			t.Errorf("%s: %v", n, err)
		}
	}
}
