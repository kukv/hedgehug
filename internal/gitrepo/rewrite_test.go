package gitrepo

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
)

var rewriteEpoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// commitAt commits files (path to content) at rewriteEpoch plus n minutes, so
// that the order of the history is the order of the calls.
func commitAt(t *testing.T, r *Repo, dir string, n int, subject string, files map[string]string) plumbing.Hash {
	t.Helper()
	var paths []string
	for p, c := range files {
		writeFile(t, dir, p, c)
		paths = append(paths, p)
	}
	h, err := r.Commit(Commit{Paths: paths, Subject: subject, Author: author, When: rewriteEpoch.Add(time.Duration(n) * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestRewriteHistory(t *testing.T) {
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	h1 := commitAt(t, r, dir, 1, "x1", map[string]string{"a.md": "a\n", "b.md": "b\n", "d/x.md": "x\n", "e/y.md": "y\n"})
	commitAt(t, r, dir, 2, "x2", map[string]string{"b.md": "b2\n"})
	h3 := commitAt(t, r, dir, 4, "x3", map[string]string{"c.md": "c\n"})
	// A merge of h3 and a side commit made on h1.
	side, err := r.Commit(Commit{Paths: nil, Subject: "side", Parents: []plumbing.Hash{h1}, Author: author, When: rewriteEpoch.Add(3 * time.Minute), AllowEmpty: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.r.Storer.SetReference(plumbing.NewHashReference(mainRef, h3)); err != nil {
		t.Fatal(err)
	}
	m, err := r.Commit(Commit{Subject: "merge", Parents: []plumbing.Hash{h3, side}, Author: author, When: rewriteEpoch.Add(5 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}

	drop := func(p string) bool { return p == "b.md" || p == "d/x.md" }
	var seen []string
	nh, n, err := r.RewriteHistory(m, Rewrite{Drop: drop, Message: func(msg string) string {
		seen = append(seen, msg)
		return strings.Replace(msg, "x", "purged", 1)
	}})
	if err != nil || n != 5 {
		t.Fatalf("RewriteHistory = %s, %d, %v; want 5 commits", nh, n, err)
	}
	if len(seen) != 5 {
		t.Errorf("Message was called %d times, want 5", len(seen))
	}
	if got, _ := r.Head(); got != m {
		t.Errorf("RewriteHistory moved HEAD to %s", got)
	}

	old, err := r.Between(m, plumbing.ZeroHash)
	if err != nil {
		t.Fatal(err)
	}
	neu, err := r.Between(nh, plumbing.ZeroHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(neu) != len(old) {
		t.Fatalf("%d commits, want %d", len(neu), len(old))
	}
	for i := range old {
		oc, _ := r.commit(old[i].Hash)
		nc, _ := r.commit(neu[i].Hash)
		if oc.Author != nc.Author || oc.Committer != nc.Committer {
			t.Errorf("commit %d: signatures changed: %v/%v -> %v/%v", i, oc.Author, oc.Committer, nc.Author, nc.Committer)
		}
		if oc.Hash == nc.Hash {
			t.Errorf("commit %d was not made again", i)
		}
		if want := strings.Replace(oc.Message, "x", "purged", 1); nc.Message != want {
			t.Errorf("commit %d: message %q, want %q", i, nc.Message, want)
		}
		for _, p := range []string{"b.md", "d/x.md"} {
			if _, err := r.ReadAt(nc.Hash, p); err == nil {
				t.Errorf("commit %d still has %s", i, p)
			}
		}
		tr, err := nc.Tree()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tr.FindEntry("d"); err == nil {
			t.Errorf("commit %d keeps the empty directory d", i)
		}
		if oc.NumParents() != nc.NumParents() {
			t.Errorf("commit %d: parents %d, want %d", i, nc.NumParents(), oc.NumParents())
		}
	}
	// The second commit changed only b.md: its tree is its parent's.
	c2, _ := r.commit(neu[1].Hash)
	c1, _ := r.commit(neu[0].Hash)
	if c2.TreeHash != c1.TreeHash {
		t.Errorf("the tree of the second commit is %s, want its parent's %s", c2.TreeHash, c1.TreeHash)
	}
	// The merge has both parents made again.
	last, _ := r.commit(nh)
	if last.NumParents() != 2 {
		t.Fatalf("merge has %d parents", last.NumParents())
	}
	for _, p := range last.ParentHashes {
		found := false
		for _, e := range neu {
			found = found || e.Hash == p
		}
		if !found {
			t.Errorf("parent %s of the merge is not a rewritten commit", p)
		}
	}
	if got, err := r.ReadAt(nh, "e/y.md"); err != nil || string(got) != "y\n" {
		t.Errorf("e/y.md = %q, %v", got, err)
	}
}

func TestRewriteHistoryNilFuncs(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	h := commitHead(t, r, dir, map[string]string{"a.md": "a\n"})
	nh, n, err := r.RewriteHistory(h, Rewrite{})
	if err != nil || n != 1 || nh != h {
		t.Errorf("RewriteHistory with nothing to do = %s, %d, %v; want the same commit %s", nh, n, err, h)
	}
}

func TestResetMain(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	commitHead(t, r, dir, map[string]string{"a.md": "a\n", "b.md": "b\n"})
	old := commitHead(t, r, dir, map[string]string{"c.md": "c\n"})
	nh, _, err := r.RewriteHistory(old, Rewrite{Drop: func(p string) bool { return p == "b.md" }})
	if err != nil {
		t.Fatal(err)
	}

	writeFile(t, dir, "a.md", "dirty\n")
	if err := r.ResetMain(nh); !errors.Is(err, ErrWorkTreeChanged) {
		t.Fatalf("ResetMain with a dirty work tree = %v", err)
	}
	if got, _ := r.Head(); got != old {
		t.Errorf("a refused ResetMain moved HEAD to %s", got)
	}
	writeFile(t, dir, "a.md", "a\n")

	if err := r.ResetMain(nh); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.Head(); got != nh {
		t.Errorf("HEAD = %s, want %s", got, nh)
	}
	if _, err := os.Stat(filepath.Join(dir, "b.md")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("b.md is left in the work tree: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "c.md")); err != nil || string(b) != "c\n" {
		t.Errorf("c.md = %q, %v", b, err)
	}
	if st, err := r.Status(); err != nil || len(st) != 0 {
		t.Errorf("Status = %v, %v; want clean", st, err)
	}
}

func TestForcePush(t *testing.T) {
	remote := newBare(t)
	a, _ := cloneTwo(t, remote)
	old := commitHead(t, a.r, a.dir, map[string]string{"x.md": "x\n", "y.md": "y\n"})
	if err := a.r.Push("origin"); err != nil {
		t.Fatal(err)
	}
	nh, _, err := a.r.RewriteHistory(old, Rewrite{Drop: func(p string) bool { return p == "y.md" }})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.r.ResetMain(nh); err != nil {
		t.Fatal(err)
	}
	if err := a.r.Push("origin"); !errors.Is(err, ErrRejected) {
		t.Fatalf("Push of a rewritten history = %v, want ErrRejected", err)
	}
	if err := a.r.ForcePush("origin"); err != nil {
		t.Fatal(err)
	}
	if got, _ := a.r.RemoteHead("origin"); got != nh {
		t.Errorf("tracking = %s, want %s", got, nh)
	}
	b, err := Open(remote)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := b.r.Reference(mainRef, true); got.Hash() != nh {
		t.Errorf("remote main = %s, want %s", got.Hash(), nh)
	}
	// Pushing the same head again is not an error.
	if err := a.r.ForcePush("origin"); err != nil {
		t.Errorf("second ForcePush = %v", err)
	}
	if err := a.r.SetTracking("origin", old); err != nil {
		t.Fatal(err)
	}
	if got, _ := a.r.RemoteHead("origin"); got != old {
		t.Errorf("tracking after SetTracking = %s, want %s", got, old)
	}
}

func TestClose(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	h := commitHead(t, r, dir, map[string]string{"a.md": "a\n"})
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	r2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := r2.ReadAt(h, "a.md"); err != nil || string(got) != "a\n" {
		t.Errorf("a.md after reopen = %q, %v", got, err)
	}
}
