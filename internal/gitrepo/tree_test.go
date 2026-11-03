package gitrepo

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/utils/merkletrie"
)

// commitFiles writes files (path → content; empty content removes the path)
// and commits them, returning the new HEAD tree.
func commitFiles(t *testing.T, r *Repo, dir string, files map[string]string) plumbing.Hash {
	t.Helper()
	var paths []string
	for p, c := range files {
		if c == "" {
			if err := removeFile(dir, p); err != nil {
				t.Fatal(err)
			}
		} else {
			writeFile(t, dir, p, c)
		}
		paths = append(paths, p)
	}
	if _, err := r.Commit(Commit{Paths: paths, Subject: "x", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	h, err := r.HeadTree()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func removeFile(dir, rel string) error {
	return os.Remove(filepath.Join(dir, filepath.FromSlash(rel)))
}

func TestDiffTrees(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	first := commitFiles(t, r, dir, map[string]string{
		"tasks/a.md":  "a1",
		"tasks/b.md":  "b1",
		"schema.yaml": "s1",
	})
	second := commitFiles(t, r, dir, map[string]string{
		"tasks/a.md":  "a2", // modified
		"tasks/b.md":  "",   // deleted
		"tasks/c.md":  "c1", // added
		"schema.yaml": "s2",
	})

	got, err := r.DiffTrees(first, second, "tasks/")
	if err != nil {
		t.Fatal(err)
	}
	slices.SortFunc(got, func(x, y Change) int {
		if x.Path < y.Path {
			return -1
		}
		if x.Path > y.Path {
			return 1
		}
		return 0
	})
	want := []Change{{Path: "tasks/a.md"}, {Path: "tasks/b.md", Deleted: true}, {Path: "tasks/c.md"}}
	if !slices.Equal(got, want) {
		t.Errorf("DiffTrees = %v, want %v", got, want)
	}

	all, err := r.DiffTrees(first, second, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 4 {
		t.Errorf("DiffTrees without prefix = %v, want 4 changes", all)
	}

	none, err := r.DiffTrees(second, second, "")
	if err != nil || len(none) != 0 {
		t.Errorf("DiffTrees of the same tree = %v, %v", none, err)
	}
}

// goGitDiff is the reference for DiffTrees: go-git's object.DiffTree, with
// each change turned into a Change and filtered by prefix. Its order is not
// bytewise path order (a directory "tasks" comes before a file "tasks.md"),
// so DiffTrees keeps this order, not a full-path sort.
func goGitDiff(t *testing.T, r *Repo, from, to plumbing.Hash, prefix string) []Change {
	t.Helper()
	a, err := r.tree(from)
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.tree(to)
	if err != nil {
		t.Fatal(err)
	}
	cs, err := object.DiffTree(a, b)
	if err != nil {
		t.Fatal(err)
	}
	var out []Change
	for _, c := range cs {
		action, err := c.Action()
		if err != nil {
			t.Fatal(err)
		}
		ch := Change{Path: c.To.Name}
		if action == merkletrie.Delete {
			ch = Change{Path: c.From.Name, Deleted: true}
		}
		if strings.HasPrefix(ch.Path, prefix) {
			out = append(out, ch)
		}
	}
	return out
}

// TestDiffTreesMatchesGoGit compares DiffTrees with go-git for every ordered
// pair of the trees a history of varied edits goes through.
func TestDiffTreesMatchesGoGit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	trees := []plumbing.Hash{commitFiles(t, r, dir, map[string]string{
		"tasks/a.md": "a1", "tasks/b.md": "b1", "tasks/sub/deep/x.md": "x1",
		"tasks.md": "t", "tasks-z/q.md": "q", "schema.yaml": "s1", "gone/g.md": "g",
		"flip": "file", "keep/k.md": "k",
	})}
	step := func(files map[string]string) {
		trees = append(trees, commitFiles(t, r, dir, files))
	}
	step(map[string]string{"tasks/c.md": "c1"})                           // add
	step(map[string]string{"tasks/a.md": "a2"})                           // change
	step(map[string]string{"tasks/b.md": ""})                             // delete
	step(map[string]string{"gone/g.md": ""})                              // a directory goes away
	step(map[string]string{"tasks/n/e/w/f.md": "f", "n.md": "n"})         // a deep directory comes
	step(map[string]string{"tasks/sub/deep/x.md": "x2", "tasks.md": "u"}) // nested change

	// A file becomes a directory, then the directory a file.
	commitPaths := func(paths ...string) {
		t.Helper()
		if _, err := r.Commit(Commit{Paths: paths, Subject: "x", Author: author, When: time.Now()}); err != nil {
			t.Fatal(err)
		}
		h, err := r.HeadTree()
		if err != nil {
			t.Fatal(err)
		}
		if h == trees[len(trees)-1] {
			t.Fatal("the commit did not change the tree")
		}
		trees = append(trees, h)
	}
	if err := removeFile(dir, "flip"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "flip/inner.md", "i")
	commitPaths("flip", "flip/inner.md")
	if err := os.RemoveAll(filepath.Join(dir, "flip")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "flip", "file again")
	commitPaths("flip")

	// Only the mode changes.
	if err := os.Chmod(filepath.Join(dir, "schema.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}
	commitPaths("schema.yaml")

	for i, from := range trees {
		for j, to := range trees {
			for _, prefix := range []string{"", "tasks/", "tasks", "flip", "nothing/"} {
				want := goGitDiff(t, r, from, to, prefix)
				got, err := r.DiffTrees(from, to, prefix)
				if err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(got, want) {
					t.Errorf("DiffTrees(%d, %d, %q) = %v, want %v", i, j, prefix, got, want)
				}
			}
		}
	}
}

func TestDiffTreesMissingTree(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	head := commitFiles(t, r, dir, map[string]string{"tasks/a.md": "a"})
	missing := plumbing.NewHash("0123456789abcdef0123456789abcdef01234567")
	if _, err := r.DiffTrees(missing, head, ""); !errors.Is(err, ErrTreeNotFound) {
		t.Errorf("err = %v, want ErrTreeNotFound", err)
	}
	if _, err := r.ReadFile(missing, "tasks/a.md"); !errors.Is(err, ErrTreeNotFound) {
		t.Errorf("ReadFile: err = %v, want ErrTreeNotFound", err)
	}
}

func TestReadFileAndFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	head := commitFiles(t, r, dir, map[string]string{
		"tasks/x/y/a.md": "hello",
		"tasks/b.md":     "b",
		"other.md":       "o",
	})
	b, err := r.ReadFile(head, "tasks/x/y/a.md")
	if err != nil || string(b) != "hello" {
		t.Errorf("ReadFile = %q, %v", b, err)
	}
	if _, err := r.ReadFile(head, "tasks/nope.md"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadFile of a missing path: err = %v, want fs.ErrNotExist", err)
	}

	var got []string
	for p, err := range r.Files(head, "tasks/") {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, p)
	}
	slices.Sort(got)
	if want := []string{"tasks/b.md", "tasks/x/y/a.md"}; !slices.Equal(got, want) {
		t.Errorf("Files = %v, want %v", got, want)
	}
	for _, c := range []struct {
		prefix string
		want   []string
	}{
		{"tasks/x/", []string{"tasks/x/y/a.md"}},
		{"tasks/x/y/a", []string{"tasks/x/y/a.md"}},
		{"tasks/b", []string{"tasks/b.md"}},
		{"o", []string{"other.md"}},
		{"tasks/nope/", nil},
		{"other.md/", nil}, // a file is no directory
	} {
		var got []string
		for p, err := range r.Files(head, c.prefix) {
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, p)
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("Files(%q) = %v, want %v", c.prefix, got, c.want)
		}
	}
}

// A directory whose tree object is gone is an error, not an empty listing.
func TestFilesMissingSubtreeObject(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	head := commitFiles(t, r, dir, map[string]string{"tasks/x/a.md": "a", "tasks/b.md": "b"})
	tr, err := r.tree(head)
	if err != nil {
		t.Fatal(err)
	}
	e, err := tr.FindEntry("tasks/x")
	if err != nil {
		t.Fatal(err)
	}
	h := e.Hash.String()
	if err := os.Remove(filepath.Join(dir, ".git", "objects", h[:2], h[2:])); err != nil {
		t.Fatal(err)
	}
	r, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, prefix := range []string{"tasks/x/", "tasks/x/y/"} {
		var failed bool
		for _, err := range r.Files(head, prefix) {
			failed = failed || err != nil
		}
		if !failed {
			t.Errorf("Files(%q) gave no error with the tree object of tasks/x gone", prefix)
		}
	}
	for p, err := range r.Files(head, "tasks/b") {
		if err != nil || p != "tasks/b.md" {
			t.Errorf("Files(tasks/b) = %q, %v", p, err)
		}
	}
}
