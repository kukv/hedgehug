package gitrepotest

import (
	"strings"
	"testing"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const mainRef = plumbing.ReferenceName("refs/heads/main")

// RewriteRemote makes main of the bare repository at bare a history of its
// own, as a git filter-repo leaves one: a single commit with no parent, with
// the author, committer and message of the old head and its tree without
// the files and directories drop names (paths from the root, slash
// separated; one that is not there is skipped). It returns the old head and
// the new one, as hex; SetRemoteMain puts the old one back.
func RewriteRemote(t testing.TB, bare string, drop ...string) (old, rewritten string) {
	t.Helper()
	r, err := git.PlainOpen(bare)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := r.Reference(mainRef, true)
	if err != nil {
		t.Fatal(err)
	}
	c, err := r.CommitObject(ref.Hash())
	if err != nil {
		t.Fatal(err)
	}
	tree := c.TreeHash
	for _, p := range drop {
		tree, _ = treeWithout(t, r, tree, p)
	}
	nc := &object.Commit{Author: c.Author, Committer: c.Committer, Message: c.Message, TreeHash: tree}
	obj := r.Storer.NewEncodedObject()
	if err := nc.Encode(obj); err != nil {
		t.Fatal(err)
	}
	h, err := r.Storer.SetEncodedObject(obj)
	if err != nil {
		t.Fatal(err)
	}
	SetRemoteMain(t, bare, h.String())
	return ref.Hash().String(), h.String()
}

// SetRemoteMain points main of the bare repository at bare to the commit
// hash (hex), which need not descend from it.
func SetRemoteMain(t testing.TB, bare, hash string) {
	t.Helper()
	r, err := git.PlainOpen(bare)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Storer.SetReference(plumbing.NewHashReference(mainRef, plumbing.NewHash(hash))); err != nil {
		t.Fatal(err)
	}
}

// treeWithout stores the tree h without the entry at path, and returns the
// new tree and whether nothing is left in it.
func treeWithout(t testing.TB, r *git.Repository, h plumbing.Hash, path string) (plumbing.Hash, bool) {
	t.Helper()
	tree, err := r.TreeObject(h)
	if err != nil {
		t.Fatal(err)
	}
	name, rest, nested := strings.Cut(path, "/")
	var entries []object.TreeEntry
	for _, e := range tree.Entries {
		switch {
		case e.Name != name:
			entries = append(entries, e)
		case nested:
			if sub, empty := treeWithout(t, r, e.Hash, rest); !empty {
				e.Hash = sub
				entries = append(entries, e)
			}
		}
	}
	obj := r.Storer.NewEncodedObject()
	if err := (&object.Tree{Entries: entries}).Encode(obj); err != nil {
		t.Fatal(err)
	}
	nh, err := r.Storer.SetEncodedObject(obj)
	if err != nil {
		t.Fatal(err)
	}
	return nh, len(entries) == 0
}
