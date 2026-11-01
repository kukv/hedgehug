package gitrepo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"

	"github.com/kukv/hedgehug/internal/atomicfile"
)

// keptHeaders drops the headers that vouch for the old commit: a signature
// or a merge tag no longer matches the commit made again.
func keptHeaders(hs []object.ExtraHeader) []object.ExtraHeader {
	var out []object.ExtraHeader
	for _, h := range hs {
		if k := strings.ToLower(h.Key); k == "mergetag" || strings.Contains(k, "sig") {
			continue
		}
		out = append(out, h)
	}
	return out
}

// Rewrite says how each commit is made again.
type Rewrite struct {
	// Drop reports whether a path leaves every tree.
	Drop func(path string) bool
	// Message gives the new message of a commit from its whole message;
	// it returns it unchanged when there is nothing to change.
	Message func(msg string) string
}

// RewriteHistory makes again every commit reachable from head, parents
// first, with trees without the dropped paths and messages as w says;
// authors, committers and their times stay (D22). A commit whose tree
// becomes its parent's stays. It writes the objects only, moves no ref,
// and returns the new head and the number of commits made again.
func (r *Repo) RewriteHistory(head plumbing.Hash, w Rewrite) (plumbing.Hash, int, error) {
	cs, err := r.Between(head, plumbing.ZeroHash)
	if err != nil {
		return plumbing.ZeroHash, 0, err
	}
	if len(cs) == 0 {
		return plumbing.ZeroHash, 0, fmt.Errorf("rewrite: no commit at %s", head)
	}
	rw := &rewriter{r: r, w: w, trees: map[treeKey]newTree{}}
	made := make(map[plumbing.Hash]plumbing.Hash, len(cs))
	for _, e := range cs {
		c, err := r.commit(e.Hash)
		if err != nil {
			return plumbing.ZeroHash, 0, err
		}
		nc := &object.Commit{
			Author:       c.Author,
			Committer:    c.Committer,
			Message:      c.Message,
			Encoding:     c.Encoding,
			ExtraHeaders: keptHeaders(c.ExtraHeaders),
		}
		if w.Message != nil {
			nc.Message = w.Message(c.Message)
		}
		nt, err := rw.tree(c.TreeHash, "")
		if err != nil {
			return plumbing.ZeroHash, 0, err
		}
		nc.TreeHash = nt.hash
		for _, p := range c.ParentHashes {
			np, ok := made[p]
			if !ok {
				return plumbing.ZeroHash, 0, fmt.Errorf("rewrite: parent %s of %s was not made first", p, c.Hash)
			}
			nc.ParentHashes = append(nc.ParentHashes, np)
		}
		if made[c.Hash], err = r.store(nc); err != nil {
			return plumbing.ZeroHash, 0, err
		}
	}
	return made[head], len(cs), nil
}

type treeKey struct {
	hash   plumbing.Hash
	prefix string
}

type newTree struct {
	hash  plumbing.Hash
	empty bool
}

type rewriter struct {
	r     *Repo
	w     Rewrite
	trees map[treeKey]newTree
}

// tree returns the tree h, found at prefix ("" or ending in a slash), made
// again without the dropped paths. A subtree that becomes empty is left out
// by its parent.
func (rw *rewriter) tree(h plumbing.Hash, prefix string) (newTree, error) {
	key := treeKey{h, prefix}
	if n, ok := rw.trees[key]; ok {
		return n, nil
	}
	t, err := rw.r.tree(h)
	if err != nil {
		return newTree{}, err
	}
	var entries []object.TreeEntry
	for _, e := range t.Entries {
		p := prefix + e.Name
		if e.Mode != filemode.Dir {
			if rw.w.Drop == nil || !rw.w.Drop(p) {
				entries = append(entries, e)
			}
			continue
		}
		sub, err := rw.tree(e.Hash, p+"/")
		if err != nil {
			return newTree{}, err
		}
		if !sub.empty {
			e.Hash = sub.hash
			entries = append(entries, e)
		}
	}
	// git sorts a directory as if its name ended in a slash.
	order := func(e object.TreeEntry) string {
		if e.Mode == filemode.Dir {
			return e.Name + "/"
		}
		return e.Name
	}
	sort.Slice(entries, func(i, j int) bool { return order(entries[i]) < order(entries[j]) })
	nh, err := rw.r.store(&object.Tree{Entries: entries})
	if err != nil {
		return newTree{}, err
	}
	n := newTree{hash: nh, empty: len(entries) == 0}
	rw.trees[key] = n
	return n, nil
}

type encoder interface {
	Encode(plumbing.EncodedObject) error
}

// store writes o to the object storage and returns its hash.
func (r *Repo) store(o encoder) (plumbing.Hash, error) {
	eo := r.r.Storer.NewEncodedObject()
	if err := o.Encode(eo); err != nil {
		return plumbing.ZeroHash, err
	}
	return r.r.Storer.SetEncodedObject(eo)
}

// ResetMain moves refs/heads/main to h, which need not descend from it, and
// makes the work tree and the index those of h. The work tree must be
// clean (ErrWorkTreeChanged otherwise). Only the paths that differ between
// HEAD and h are written or removed: ignored and untracked files are never
// touched. The caller holds the lock of the store.
func (r *Repo) ResetMain(h plumbing.Hash) (err error) {
	unlock, err := r.lockIndex()
	if err != nil {
		return err
	}
	defer unlock()
	ref, err := r.r.Storer.Reference(plumbing.HEAD)
	if err != nil {
		return err
	}
	if ref.Type() != plumbing.SymbolicReference || ref.Target() != mainRef {
		return fmt.Errorf("%w: HEAD is not on main", ErrWorkTreeChanged)
	}
	st, err := r.Status()
	if err != nil {
		return err
	}
	if len(st) > 0 {
		return fmt.Errorf("%w: %s differs from HEAD", ErrWorkTreeChanged, st[0].Path)
	}
	from, err := r.HeadTree()
	if err != nil {
		return err
	}
	to, err := r.TreeOf(h)
	if err != nil {
		return err
	}
	changes, err := r.DiffTrees(from, to, "")
	if err != nil {
		return err
	}
	paths := make([]string, len(changes))
	data := make([][]byte, len(changes))
	for i, c := range changes {
		if err := checkWorkPath(c.Path); err != nil {
			return err
		}
		paths[i] = c.Path
		if !c.Deleted {
			if data[i], err = r.ReadFile(to, c.Path); err != nil {
				return err
			}
		}
	}
	before, err := r.r.Storer.Index()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if rerr := r.r.Storer.SetIndex(before); rerr != nil {
				err = errors.Join(err, rerr)
			}
		}
	}()
	for i, c := range changes {
		abs := filepath.Join(r.dir, filepath.FromSlash(c.Path))
		if c.Deleted {
			err = os.Remove(abs)
			if errors.Is(err, fs.ErrNotExist) {
				err = nil
			}
			r.removeEmptyDirs(filepath.Dir(abs))
		} else if err = os.MkdirAll(filepath.Dir(abs), 0o750); err == nil {
			err = atomicfile.WriteFile(abs, data[i])
		}
		if err != nil {
			return err
		}
	}
	w, err := r.r.Worktree()
	if err != nil {
		return err
	}
	if err = r.stage(w, paths); err != nil {
		return err
	}
	return r.r.Storer.SetReference(plumbing.NewHashReference(mainRef, h))
}

// removeEmptyDirs removes dir and its parents up to the repository root for
// as long as they are empty.
func (r *Repo) removeEmptyDirs(dir string) {
	for dir != r.dir && strings.HasPrefix(dir, r.dir) {
		if os.Remove(dir) != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

// ForcePush sends main to the remote even when the remote is not its
// ancestor, and sets refs/remotes/<remote>/main. A non-zero expect is a
// lease: the push is ErrRejected unless the remote's main is expect, so a
// commit pushed since expect was fetched is not lost.
func (r *Repo) ForcePush(remote string, expect plumbing.Hash) error {
	return r.push(remote, true, expect)
}

// SetTracking sets refs/remotes/<remote>/main to h (D23: put it back).
func (r *Repo) SetTracking(remote string, h plumbing.Hash) error {
	return r.r.Storer.SetReference(plumbing.NewHashReference(trackingRef(remote), h))
}

// Close lets go of the files the repository holds open (the object
// storage), so its directory can be renamed on Windows.
func (r *Repo) Close() error {
	if c, ok := r.r.Storer.(interface{ Close() error }); ok {
		return c.Close()
	}
	return nil
}

// HasSameCommit tells whether the history of from holds a commit with the
// same author and committer (name, email and time) as the commit like. A
// history rewrite keeps these (D22), so it finds where like went (D33). Both
// commits must be in the repository.
func (r *Repo) HasSameCommit(from, like plumbing.Hash) (bool, error) {
	want, err := r.commit(like)
	if err != nil {
		return false, err
	}
	start, err := r.commit(from)
	if err != nil {
		return false, err
	}
	same := func(a, b object.Signature) bool {
		return a.Name == b.Name && a.Email == b.Email && a.When.Equal(b.When)
	}
	found := false
	err = object.NewCommitPreorderIter(start, nil, nil).ForEach(func(c *object.Commit) error {
		if same(c.Author, want.Author) && same(c.Committer, want.Committer) {
			found = true
			return storer.ErrStop
		}
		return nil
	})
	return found, err
}
