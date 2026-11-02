package gitrepo

import (
	"fmt"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// plumbInit opens a fresh repository for tests that write objects directly.
func plumbInit(t *testing.T) *Repo {
	t.Helper()
	r, err := Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// putTree stores the tree of files (path to content) and returns its hash.
// Nothing outside files is in it.
func putTree(t *testing.T, r *Repo, files map[string]string) plumbing.Hash {
	t.Helper()
	store := func(o plumbing.EncodedObject) plumbing.Hash {
		h, err := r.r.Storer.SetEncodedObject(o)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	var build func(prefix string) plumbing.Hash
	build = func(prefix string) plumbing.Hash {
		tree := &object.Tree{}
		dirs := map[string]bool{}
		for p, content := range files {
			rest, ok := strings.CutPrefix(p, prefix)
			if !ok {
				continue
			}
			if dir, _, nested := strings.Cut(rest, "/"); nested {
				dirs[dir] = true
				continue
			}
			o := r.r.Storer.NewEncodedObject()
			o.SetType(plumbing.BlobObject)
			w, err := o.Writer()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write([]byte(content)); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			tree.Entries = append(tree.Entries, object.TreeEntry{Name: rest, Mode: filemode.Regular, Hash: store(o)})
		}
		for d := range dirs {
			tree.Entries = append(tree.Entries, object.TreeEntry{Name: d, Mode: filemode.Dir, Hash: build(prefix + d + "/")})
		}
		// git orders entries as if directory names ended in "/".
		key := func(e object.TreeEntry) string {
			if e.Mode == filemode.Dir {
				return e.Name + "/"
			}
			return e.Name
		}
		slices.SortFunc(tree.Entries, func(a, b object.TreeEntry) int { return strings.Compare(key(a), key(b)) })
		o := r.r.Storer.NewEncodedObject()
		if err := tree.Encode(o); err != nil {
			t.Fatal(err)
		}
		return store(o)
	}
	return build("")
}

// plumbClock hands out a distinct author time to each commit the helper
// makes, across tests that run in parallel.
var plumbClock atomic.Int64

// plumbCommit stores a commit with the given parents and exactly files as
// its tree. Paths not in files are gone.
func plumbCommit(t *testing.T, r *Repo, parents []plumbing.Hash, files map[string]string) plumbing.Hash {
	t.Helper()
	when := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(plumbClock.Add(1)) * time.Minute)
	sig := object.Signature{Name: "t", Email: "t@example.com", When: when}
	c := &object.Commit{
		Author: sig, Committer: sig, Message: "x\n",
		TreeHash: putTree(t, r, files), ParentHashes: parents,
	}
	o := r.r.Storer.NewEncodedObject()
	if err := c.Encode(o); err != nil {
		t.Fatal(err)
	}
	h, err := r.r.Storer.SetEncodedObject(o)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestChangedFiles(t *testing.T) {
	t.Parallel()
	r := plumbInit(t)
	root := plumbCommit(t, r, nil, map[string]string{"tasks/a.md": "1", "schema.yaml": "s"})
	c1 := plumbCommit(t, r, []plumbing.Hash{root}, map[string]string{"tasks/a.md": "2", "comments/x/y.md": "c", "schema.yaml": "s"})
	side := plumbCommit(t, r, []plumbing.Hash{root}, map[string]string{"tasks/a.md": "1", "tasks/b.md": "b", "schema.yaml": "s"})
	// A merge that takes each side's content changes nothing of its own.
	clean := plumbCommit(t, r, []plumbing.Hash{c1, side}, map[string]string{"tasks/a.md": "2", "comments/x/y.md": "c", "tasks/b.md": "b", "schema.yaml": "s"})
	// A merge whose a.md differs from both parents changed a.md.
	both := plumbCommit(t, r, []plumbing.Hash{root}, map[string]string{"tasks/a.md": "3", "schema.yaml": "s"})
	mixed := plumbCommit(t, r, []plumbing.Hash{c1, both}, map[string]string{"tasks/a.md": "4", "comments/x/y.md": "c", "schema.yaml": "s"})
	// A file replaced by a directory of the same name.
	swapped := plumbCommit(t, r, []plumbing.Hash{c1}, map[string]string{"tasks/a.md/z": "z", "comments/x/y.md": "c", "schema.yaml": "s"})
	for _, c := range []struct {
		name string
		h    plumbing.Hash
		want []string
	}{
		{"root", root, []string{"schema.yaml", "tasks/a.md"}},
		{"one parent", c1, []string{"comments/x/y.md", "tasks/a.md"}},
		{"clean merge", clean, nil},
		{"merge that made new content", mixed, []string{"tasks/a.md"}},
		{"file became a directory", swapped, []string{"tasks/a.md", "tasks/a.md/z"}},
	} {
		e, got, err := r.ChangedFiles(c.h)
		if err != nil {
			t.Fatal(err)
		}
		if e.Hash != c.h || !slices.Equal(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestEntry(t *testing.T) {
	t.Parallel()
	r := plumbInit(t)
	h := plumbCommit(t, r, nil, map[string]string{"f": "1"})
	e, err := r.Entry(h)
	if err != nil || e.Hash != h || e.Subject != "x" {
		t.Fatalf("Entry: %+v, %v", e, err)
	}
	if _, err := r.Entry(plumbing.NewHash("1111111111111111111111111111111111111111")); err == nil {
		t.Fatal("Entry of a missing commit succeeded")
	}
}

func TestWalkFirstParentsStops(t *testing.T) {
	t.Parallel()
	r := plumbInit(t)
	var h plumbing.Hash
	for i := range 5 {
		var parents []plumbing.Hash
		if !h.IsZero() {
			parents = []plumbing.Hash{h}
		}
		h = plumbCommit(t, r, parents, map[string]string{"f": fmt.Sprint(i)})
	}
	n := 0
	if err := r.WalkFirstParents(h, func(LogEntry) bool { n++; return n < 2 }); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("visited %d commits, want 2", n)
	}
	if err := r.WalkFirstParents(plumbing.ZeroHash, func(LogEntry) bool { t.Fatal("called for the zero hash"); return true }); err != nil {
		t.Fatal(err)
	}
	all, err := r.LogFrom(h, 0)
	if err != nil || len(all) != 5 {
		t.Fatalf("LogFrom: %d entries, %v", len(all), err)
	}
	two, err := r.LogFrom(h, 2)
	if err != nil || len(two) != 2 {
		t.Fatalf("LogFrom limit 2: %d entries, %v", len(two), err)
	}
}
