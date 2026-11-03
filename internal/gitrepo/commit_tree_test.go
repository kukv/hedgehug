package gitrepo

import (
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// goGitTree returns the tree go-git's Worktree.Commit makes from the index
// of the repository at dir, which is what Repo.Commit made before (spec
// gogit-fixed-cost 3). HEAD is put back after.
func goGitTree(t *testing.T, dir string) plumbing.Hash {
	t.Helper()
	g, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	head, err := g.Head()
	if err != nil {
		t.Fatal(err)
	}
	w, err := g.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	sig := &object.Signature{Name: "oracle", Email: "oracle@example.com", When: time.Now()}
	h, err := w.Commit("oracle", &git.CommitOptions{Author: sig, AllowEmptyCommits: true})
	if err != nil {
		t.Fatal(err)
	}
	c, err := g.CommitObject(h)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Storer.SetReference(plumbing.NewHashReference(head.Name(), head.Hash())); err != nil {
		t.Fatal(err)
	}
	return c.TreeHash
}

func TestCommitMakesTheTreeGoGitMakes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	steps := []struct {
		name  string
		files map[string]string // empty content removes the path
	}{
		{"first", map[string]string{"schema.yaml": "s1", "tasks/a/task.md": "a1", "tasks/b/task.md": "b1"}},
		{"modify", map[string]string{"tasks/a/task.md": "a2"}},
		{"add deep", map[string]string{"tasks/a/comments/c1.md": "c1", "x/y/z/w.md": "w"}},
		{"several", map[string]string{"schema.yaml": "s2", "tasks/b/task.md": "b2", "tasks/c/task.md": "c1"}},
		{"delete empties a directory", map[string]string{"tasks/a/comments/c1.md": ""}},
		{"delete empties a chain", map[string]string{"x/y/z/w.md": ""}},
		// "a.md" < "a/" < "a0.md" in git's order, which compares a
		// directory's name with a "/" after it.
		{"order", map[string]string{"tasks/a.md": "f", "tasks/a0.md": "g", "tasks/a-b/task.md": "h"}},
		{"remove a file", map[string]string{"tasks/a.md": ""}},
		{"a directory where the file was", map[string]string{"tasks/a.md/task.md": "i"}},
		{"delete everything", map[string]string{
			"schema.yaml": "", "tasks/a/task.md": "", "tasks/b/task.md": "", "tasks/c/task.md": "",
			"tasks/a0.md": "", "tasks/a-b/task.md": "", "tasks/a.md/task.md": "",
		}},
	}
	for _, s := range steps {
		got := commitFiles(t, r, dir, s.files)
		if want := goGitTree(t, dir); got != want {
			t.Fatalf("%s: tree %s, go-git makes %s", s.name, got, want)
		}
	}
}

// A merge commit with no path has the tree of HEAD and both parents.
func TestCommitMergeKeepsTheTree(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	first := commitHead(t, r, dir, map[string]string{"a.md": "1"})
	second := commitHead(t, r, dir, map[string]string{"b.md": "2"})
	tree, err := r.HeadTree()
	if err != nil {
		t.Fatal(err)
	}
	h, err := r.Commit(Commit{Parents: []plumbing.Hash{second, first}, Subject: "merge", Author: author, When: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	c, err := r.r.CommitObject(h)
	if err != nil {
		t.Fatal(err)
	}
	if c.TreeHash != tree || len(c.ParentHashes) != 2 || c.ParentHashes[0] != second || c.ParentHashes[1] != first {
		t.Errorf("merge: tree %s parents %v; want tree %s parents [%s %s]", c.TreeHash, c.ParentHashes, tree, second, first)
	}
	if got := mustHead(t, r); got != h {
		t.Errorf("HEAD = %s, want the merge %s", got, h)
	}
}

func TestCommitAllowEmptyHasHeadAsParent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	before := commitHead(t, r, dir, map[string]string{"a.md": "1"})
	h, err := r.Commit(Commit{AllowEmpty: true, Subject: "replay", Author: author, When: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	c, err := r.r.CommitObject(h)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.ParentHashes) != 1 || c.ParentHashes[0] != before {
		t.Errorf("parents = %v, want [%s]", c.ParentHashes, before)
	}
}

// Only c.Paths are committed: a path staged by hand stays out (spec
// gogit-fixed-cost 3.2).
func TestCommitLeavesOutPathsStagedByHand(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	commitFiles(t, r, dir, map[string]string{"a.md": "1"})
	writeFile(t, dir, "other.md", "by hand")
	w, err := r.r.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Add("other.md"); err != nil {
		t.Fatal(err)
	}
	commitFiles(t, r, dir, map[string]string{"a.md": "2"})
	c, err := r.r.CommitObject(mustHead(t, r))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.File("other.md"); err == nil {
		t.Error("other.md, staged by hand, was committed")
	}
}

// The author's name and email lose what git does not allow in them, as
// go-git's Worktree.Commit did.
func TestCommitSanitizesTheAuthor(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	writeFile(t, dir, "a.md", "1")
	h, err := r.Commit(Commit{Paths: []string{"a.md"}, Subject: "x", Author: Signature{Name: "A <b>\n", Email: "<a@example.com>"}, When: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	c, err := r.r.CommitObject(h)
	if err != nil {
		t.Fatal(err)
	}
	if c.Author.Name != "A b" || c.Author.Email != "a@example.com" || c.Committer.Name != "A b" || c.Committer.Email != "a@example.com" {
		t.Errorf("author %q <%q>, committer %q <%q>", c.Author.Name, c.Author.Email, c.Committer.Name, c.Committer.Email)
	}
}
