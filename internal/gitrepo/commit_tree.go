package gitrepo

import (
	"errors"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// commitStaged commits c, whose paths are staged already, and moves HEAD to
// it. The tree is HEAD's with c.Paths as in the index; only the trees above
// c.Paths are made again. go-git's Worktree.Commit builds every tree of the
// index and asks the disk for each whether it exists, about 5,000 opens per
// commit on a store of 5,000 tasks (spec gogit-fixed-cost 3). The index
// outside c.Paths is taken to be HEAD's tree, as it is after every hh write.
func (r *Repo) commitStaged(c Commit) (plumbing.Hash, error) {
	head, err := r.r.Head()
	if err != nil && !errors.Is(err, plumbing.ErrReferenceNotFound) {
		return plumbing.ZeroHash, err
	}
	var base plumbing.Hash
	parents := c.Parents
	if head != nil {
		hc, err := r.r.CommitObject(head.Hash())
		if err != nil {
			return plumbing.ZeroHash, err
		}
		base = hc.TreeHash
		if len(parents) == 0 {
			parents = []plumbing.Hash{head.Hash()}
		}
	}
	idx, err := r.r.Storer.Index()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	edits := make(map[string]*index.Entry, len(c.Paths))
	for _, p := range c.Paths {
		e, err := idx.Entry(p)
		switch {
		case err == nil:
			edits[p] = e
		case errors.Is(err, index.ErrEntryNotFound):
			edits[p] = nil
		default:
			return plumbing.ZeroHash, err
		}
	}
	tree, empty, err := r.editTree(base, edits, true)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	if len(parents) < 2 && !c.AllowEmpty {
		if len(parents) == 0 && empty {
			return plumbing.ZeroHash, ErrNothingToCommit
		}
		if len(parents) > 0 {
			pc, err := r.r.CommitObject(parents[0])
			if err != nil {
				return plumbing.ZeroHash, err
			}
			if pc.TreeHash == tree {
				return plumbing.ZeroHash, ErrNothingToCommit
			}
		}
	}
	sig := object.Signature{Name: badSignature.ReplaceAllString(c.Author.Name, ""), Email: badSignature.ReplaceAllString(c.Author.Email, ""), When: c.When}
	commit := &object.Commit{Author: sig, Committer: sig, Message: message(c), TreeHash: tree, ParentHashes: parents}
	o := r.r.Storer.NewEncodedObject()
	if err := commit.Encode(o); err != nil {
		return plumbing.ZeroHash, err
	}
	h, err := r.r.Storer.SetEncodedObject(o)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return h, r.moveHead(h)
}

// badSignature is what git does not allow in a name or an email; go-git's
// Worktree.Commit drops it too.
var badSignature = regexp.MustCompile(`[<>\n]`)

// moveHead points HEAD, or the branch it is on, at h.
func (r *Repo) moveHead(h plumbing.Hash) error {
	ref, err := r.r.Storer.Reference(plumbing.HEAD)
	if err != nil {
		return err
	}
	name := plumbing.HEAD
	if ref.Type() != plumbing.HashReference {
		name = ref.Target()
	}
	return r.r.Storer.SetReference(plumbing.NewHashReference(name, h))
}

// editTree returns the tree base (zero: an empty tree) with edits made, and
// whether it is empty. edits maps a path relative to base to its index entry,
// or to nil to remove it. Only the trees on the edited paths are read and
// written. An empty tree is dropped from its parent; only the root is
// written when empty.
func (r *Repo) editTree(base plumbing.Hash, edits map[string]*index.Entry, root bool) (plumbing.Hash, bool, error) {
	var entries []object.TreeEntry
	if !base.IsZero() {
		t, err := r.r.TreeObject(base)
		if err != nil {
			return plumbing.ZeroHash, false, err
		}
		entries = slices.Clone(t.Entries)
	}
	at := func(name string) int {
		return slices.IndexFunc(entries, func(e object.TreeEntry) bool { return e.Name == name })
	}
	subs := map[string]map[string]*index.Entry{}
	for p, e := range edits {
		name, rest, nested := strings.Cut(p, "/")
		if nested {
			if subs[name] == nil {
				subs[name] = map[string]*index.Entry{}
			}
			subs[name][rest] = e
			continue
		}
		if i := at(name); i >= 0 {
			entries = slices.Delete(entries, i, i+1)
		}
		if e != nil {
			entries = append(entries, object.TreeEntry{Name: name, Mode: e.Mode, Hash: e.Hash})
		}
	}
	for name, sub := range subs {
		i := at(name)
		var cur plumbing.Hash
		if i >= 0 && entries[i].Mode == filemode.Dir {
			cur = entries[i].Hash
		}
		h, empty, err := r.editTree(cur, sub, false)
		if err != nil {
			return plumbing.ZeroHash, false, err
		}
		dir := object.TreeEntry{Name: name, Mode: filemode.Dir, Hash: h}
		switch {
		case !empty && i >= 0:
			entries[i] = dir
		case !empty:
			entries = append(entries, dir)
		case i >= 0 && entries[i].Mode == filemode.Dir:
			entries = slices.Delete(entries, i, i+1)
		}
	}
	if len(entries) == 0 && !root {
		return plumbing.ZeroHash, true, nil
	}
	// git orders a directory as if its name ended in "/".
	key := func(e object.TreeEntry) string {
		if e.Mode == filemode.Dir {
			return e.Name + "/"
		}
		return e.Name
	}
	sort.Slice(entries, func(i, j int) bool { return key(entries[i]) < key(entries[j]) })
	o := r.r.Storer.NewEncodedObject()
	if err := (&object.Tree{Entries: entries}).Encode(o); err != nil {
		return plumbing.ZeroHash, false, err
	}
	if o.Hash() == base {
		return base, len(entries) == 0, nil // unchanged; already stored
	}
	h, err := r.r.Storer.SetEncodedObject(o)
	return h, len(entries) == 0, err
}
