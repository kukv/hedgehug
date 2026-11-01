package gitrepo

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var author = Signature{Name: "Test", Email: "test@example.com"}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func head(t *testing.T, dir string) *object.Commit {
	t.Helper()
	r, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	c, err := r.CommitObject(ref.Hash())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func files(t *testing.T, c *object.Commit) []string {
	t.Helper()
	tree, err := c.Tree()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	if err := tree.Files().ForEach(func(f *object.File) error {
		out = append(out, f.Name)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	slices.Sort(out)
	return out
}

func TestCommitOnlyNamedPaths(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "tasks/A/B/one.md", "one")
	writeFile(t, dir, "other.md", "not committed")
	when := time.Date(2026, 10, 30, 12, 0, 0, 0, time.FixedZone("", 9*60*60))
	if _, err := r.Commit(Commit{
		Paths:    []string{"tasks/A/B/one.md"},
		Subject:  "hedgehug: add 01JB x",
		Trailers: []Trailer{{"Hedgehug-Op", "add"}, {"Hedgehug-Task", "01JB"}},
		Author:   author,
		When:     when,
	}); err != nil {
		t.Fatal(err)
	}

	c := head(t, dir)
	if got := files(t, c); !slices.Equal(got, []string{"tasks/A/B/one.md"}) {
		t.Errorf("files = %v", got)
	}
	if want := "hedgehug: add 01JB x\n\nHedgehug-Op: add\nHedgehug-Task: 01JB\n"; c.Message != want {
		t.Errorf("message = %q, want %q", c.Message, want)
	}
	if c.Author.Name != "Test" || c.Author.Email != "test@example.com" || !c.Author.When.Equal(when) {
		t.Errorf("author = %+v", c.Author)
	}
	if c.Committer.Name != "Test" || c.Committer.Email != "test@example.com" {
		t.Errorf("committer = %+v", c.Committer)
	}

	ref, err := r.r.Head()
	if err != nil {
		t.Fatal(err)
	}
	if ref.Name().Short() != "main" {
		t.Errorf("branch = %s, want main", ref.Name().Short())
	}
}

func TestCommitRemovedPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "a.md", "a")
	writeFile(t, dir, "b.md", "b")
	if _, err := r.Commit(Commit{Paths: []string{"a.md", "b.md"}, Subject: "two", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Commit(Commit{Paths: []string{"a.md"}, Subject: "rm", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	c := head(t, dir)
	if got := files(t, c); !slices.Equal(got, []string{"b.md"}) {
		t.Errorf("files = %v", got)
	}
	if c.NumParents() != 1 {
		t.Errorf("parents = %d", c.NumParents())
	}
}

func TestCommitNothing(t *testing.T) {
	t.Parallel()
	r, err := Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Commit(Commit{Subject: "empty", Author: author, When: time.Now()}); !errors.Is(err, ErrNothingToCommit) {
		t.Errorf("err = %v, want ErrNothingToCommit", err)
	}
}

func TestFailedCommitKeepsIndex(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "a.md", "one")
	if _, err := r.Commit(Commit{Paths: []string{"a.md"}, Subject: "one", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "a.md", "two")
	// A NUL byte makes Lstat fail with something other than "not exist",
	// after a.md has already been staged.
	if _, err := r.Commit(Commit{Paths: []string{"a.md", "bad\x00.md"}, Subject: "two", Author: author, When: time.Now()}); err == nil {
		t.Fatal("Commit with an invalid path succeeded")
	}

	want, err := head(t, dir).File("a.md")
	if err != nil {
		t.Fatal(err)
	}
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := repo.Storer.Index()
	if err != nil {
		t.Fatal(err)
	}
	e, err := idx.Entry("a.md")
	if err != nil {
		t.Fatal(err)
	}
	if e.Hash != want.Hash {
		t.Errorf("index a.md = %s, want HEAD's %s", e.Hash, want.Hash)
	}
}

func TestOpen(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := Init(dir); err != nil {
		t.Fatal(err)
	}
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "a.md", "a")
	if _, err := r.Commit(Commit{Paths: []string{"a.md"}, Subject: "a", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(t.TempDir()); err == nil {
		t.Error("Open of a directory without a repository succeeded")
	}
}
