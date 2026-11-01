package gitrepo

import (
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// putCommit stores a commit with the given identities on top of parent.
func putCommit(t *testing.T, r *Repo, parent plumbing.Hash, author, committer object.Signature) plumbing.Hash {
	t.Helper()
	tree, err := r.HeadTree()
	if err != nil {
		t.Fatal(err)
	}
	c := &object.Commit{Author: author, Committer: committer, Message: "x\n", TreeHash: tree}
	if !parent.IsZero() {
		c.ParentHashes = []plumbing.Hash{parent}
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

func TestHasSameCommit(t *testing.T) {
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	commitFiles(t, r, dir, map[string]string{"a.md": "a\n"})
	at := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	sig := func(name string, when time.Time) object.Signature {
		return object.Signature{Name: name, Email: name + "@x", When: when}
	}
	root := putCommit(t, r, plumbing.ZeroHash, sig("root", at), sig("root", at))
	tracked := putCommit(t, r, root, sig("u", at), sig("u", at.Add(time.Second)))
	// Only the committer time differs.
	late := putCommit(t, r, root, sig("u", at), sig("u", at.Add(2*time.Second)))
	// The same identities on another commit, as a rewrite makes them.
	twin := putCommit(t, r, late, sig("u", at), sig("u", at.Add(time.Second)))
	onTop := putCommit(t, r, twin, sig("v", at), sig("v", at))
	// Only the author differs.
	other := putCommit(t, r, root, sig("w", at), sig("u", at.Add(time.Second)))

	for _, tc := range []struct {
		name string
		from plumbing.Hash
		want bool
	}{
		{"the commit itself", tracked, true},
		{"a twin below the head", onTop, true},
		{"committer time differs", late, false},
		{"author differs", other, false},
	} {
		got, err := r.HasSameCommit(tc.from, tracked)
		if err != nil || got != tc.want {
			t.Errorf("%s: HasSameCommit = %v, %v; want %v", tc.name, got, err, tc.want)
		}
	}
}
