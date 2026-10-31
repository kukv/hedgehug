package gitrepo

import (
	"bytes"
	"container/heap"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/atomicfile"
)

// ErrWorkTreeChanged means the repository is no longer what the caller saw
// when it chose the commit to fast-forward to: HEAD moved or left main, or a
// path to update was changed. The caller looks again and retries.
var ErrWorkTreeChanged = errors.New("the store changed during the fast-forward")

func (r *Repo) commit(h plumbing.Hash) (*object.Commit, error) {
	c, err := r.r.CommitObject(h)
	if err != nil {
		return nil, fmt.Errorf("commit %s: %w", h, err)
	}
	return c, nil
}

// MergeBase returns a newest common ancestor of a and b, or the zero hash
// when they have none or either is zero. When there are several, as after
// criss-cross merges, the one with the smallest hash is returned, so that
// MergeBase(a, b) is MergeBase(b, a).
func (r *Repo) MergeBase(a, b plumbing.Hash) (plumbing.Hash, error) {
	if a.IsZero() || b.IsZero() {
		return plumbing.ZeroHash, nil
	}
	ca, err := r.commit(a)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	cb, err := r.commit(b)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	bases, err := ca.MergeBase(cb)
	if err != nil || len(bases) == 0 {
		return plumbing.ZeroHash, err
	}
	best := bases[0].Hash
	for _, c := range bases[1:] {
		if bytes.Compare(c.Hash[:], best[:]) < 0 {
			best = c.Hash
		}
	}
	return best, nil
}

// IsAncestor reports whether a is b or one of its ancestors. The zero hash,
// no commit, comes before every commit; nothing but itself comes before it.
func (r *Repo) IsAncestor(a, b plumbing.Hash) (bool, error) {
	switch {
	case a == b || a.IsZero():
		return true, nil
	case b.IsZero():
		return false, nil
	}
	ca, err := r.commit(a)
	if err != nil {
		return false, err
	}
	cb, err := r.commit(b)
	if err != nil {
		return false, err
	}
	return ca.IsAncestor(cb)
}

// TreeOf returns the tree of commit; the zero commit has the zero tree.
func (r *Repo) TreeOf(commit plumbing.Hash) (plumbing.Hash, error) {
	if commit.IsZero() {
		return plumbing.ZeroHash, nil
	}
	c, err := r.commit(commit)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return c.TreeHash, nil
}

// ReadAt returns the content of path at commit. A missing path, or the zero
// commit, is fs.ErrNotExist.
func (r *Repo) ReadAt(commit plumbing.Hash, path string) ([]byte, error) {
	if commit.IsZero() {
		return nil, fmt.Errorf("%s: %w", path, fs.ErrNotExist)
	}
	tree, err := r.TreeOf(commit)
	if err != nil {
		return nil, err
	}
	return r.ReadFile(tree, path)
}

// Between returns the commits reachable from from but not from base, through
// every parent, in the order of the history: a commit never comes before its
// parents. Among commits that can go next, the oldest by author time goes
// first, then the smallest hash, so the order does not depend on the order of
// a merge's parents. A zero base excludes nothing.
func (r *Repo) Between(from, base plumbing.Hash) ([]LogEntry, error) {
	cs, err := r.side(from, base)
	if err != nil {
		return nil, err
	}
	out := make([]LogEntry, len(cs))
	for i, c := range cs {
		out[i] = entry(c)
	}
	return out, nil
}

// LastChange returns the latest commit by author time between base and from,
// as Between counts them, that changed path: one whose content of path
// differs from its parent's, or, for a merge, from every parent's, so that a
// merge that takes one side's content is not a change of its own. Of commits
// of the same time, the one later in Between's order wins. ok is false when
// no commit changed it.
func (r *Repo) LastChange(from, base plumbing.Hash, path string) (e LogEntry, ok bool, err error) {
	cs, err := r.side(from, base)
	if err != nil {
		return LogEntry{}, false, err
	}
	var best *object.Commit
	for _, c := range cs {
		changed, err := r.changes(c, path)
		if err != nil {
			return LogEntry{}, false, err
		}
		if changed && (best == nil || !c.Author.When.Before(best.Author.When)) {
			best = c
		}
	}
	if best == nil {
		return LogEntry{}, false, nil
	}
	return entry(best), true, nil
}

// side returns the commits of Between, in its order. Commits never change, so
// the result is kept for the next call with the same from and base; callers
// must not modify it.
func (r *Repo) side(from, base plumbing.Hash) ([]*object.Commit, error) {
	if from.IsZero() {
		return nil, nil
	}
	key := [2]plumbing.Hash{from, base}
	r.mu.Lock()
	cs, ok := r.sides[key]
	r.mu.Unlock()
	if ok {
		return cs, nil
	}
	cs, err := r.walkSide(from, base)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	if r.sides == nil {
		r.sides = map[[2]plumbing.Hash][]*object.Commit{}
	}
	r.sides[key] = cs
	r.mu.Unlock()
	return cs, nil
}

func (r *Repo) walkSide(from, base plumbing.Hash) ([]*object.Commit, error) {
	seen := map[plumbing.Hash]bool{}
	if !base.IsZero() {
		cb, err := r.commit(base)
		if err != nil {
			return nil, err
		}
		err = object.NewCommitPreorderIter(cb, nil, nil).ForEach(func(c *object.Commit) error {
			seen[c.Hash] = true
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	cf, err := r.commit(from)
	if err != nil {
		return nil, err
	}
	in := map[plumbing.Hash]*object.Commit{}
	err = object.NewCommitPreorderIter(cf, seen, nil).ForEach(func(c *object.Commit) error {
		in[c.Hash] = c
		return nil
	})
	if err != nil {
		return nil, err
	}
	// A commit is ready once all its parents in the side are out.
	waiting := map[plumbing.Hash]int{}
	children := map[plumbing.Hash][]*object.Commit{}
	ready := &commitHeap{}
	for _, c := range in {
		for _, p := range c.ParentHashes {
			if _, ok := in[p]; ok {
				waiting[c.Hash]++
				children[p] = append(children[p], c)
			}
		}
		if waiting[c.Hash] == 0 {
			heap.Push(ready, c)
		}
	}
	out := make([]*object.Commit, 0, len(in))
	for ready.Len() > 0 {
		c := heap.Pop(ready).(*object.Commit)
		out = append(out, c)
		for _, ch := range children[c.Hash] {
			if waiting[ch.Hash]--; waiting[ch.Hash] == 0 {
				heap.Push(ready, ch)
			}
		}
	}
	return out, nil
}

// commitHeap orders commits by author time, then hash.
type commitHeap []*object.Commit

func (h commitHeap) Len() int { return len(h) }

func (h commitHeap) Less(i, j int) bool {
	if c := h[i].Author.When.Compare(h[j].Author.When); c != 0 {
		return c < 0
	}
	return bytes.Compare(h[i].Hash[:], h[j].Hash[:]) < 0
}

func (h commitHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *commitHeap) Push(x any) { *h = append(*h, x.(*object.Commit)) }

func (h *commitHeap) Pop() any {
	old := *h
	c := old[len(old)-1]
	*h = old[:len(old)-1]
	return c
}

// changes reports whether c changed path against all of its parents.
func (r *Repo) changes(c *object.Commit, path string) (bool, error) {
	h, err := blobAt(c, path)
	if err != nil {
		return false, err
	}
	if c.NumParents() == 0 {
		return !h.IsZero(), nil
	}
	for _, ph := range c.ParentHashes {
		p, err := r.commit(ph)
		if err != nil {
			return false, err
		}
		ph, err := blobAt(p, path)
		if err != nil {
			return false, err
		}
		if ph == h {
			return false, nil
		}
	}
	return true, nil
}

// blobAt returns the hash of path in c, or the zero hash when it is missing.
func blobAt(c *object.Commit, path string) (plumbing.Hash, error) {
	t, err := c.Tree()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	e, err := t.FindEntry(path)
	if errors.Is(err, object.ErrEntryNotFound) || errors.Is(err, object.ErrDirectoryNotFound) {
		return plumbing.ZeroHash, nil
	}
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return e.Hash, nil
}

// FastForward moves refs/heads/main to h and makes paths, in the work tree
// and in the index, what they are in h: a path in h is written, one that is
// not is removed. Nothing else in the work tree is touched.
//
// Under the index lock it first makes sure the repository is still as the
// caller saw it: HEAD is on main, h descends from it, and every path on disk
// is as in HEAD or already as in h. Otherwise it returns ErrWorkTreeChanged.
// Every path is read from h before anything is written, so a commit or object
// that is missing changes nothing either.
func (r *Repo) FastForward(h plumbing.Hash, paths []string) (err error) {
	for _, p := range paths {
		if err := checkWorkPath(p); err != nil {
			return err
		}
	}
	unlock, err := r.lockIndex()
	if err != nil {
		return err
	}
	defer unlock()
	tree, err := r.TreeOf(h)
	if err != nil {
		return err
	}
	if tree.IsZero() {
		return errors.New("fast-forward to the zero commit")
	}
	data := make([][]byte, len(paths))
	inTree := make([]bool, len(paths))
	for i, p := range paths {
		b, err := r.ReadFile(tree, p)
		switch {
		case err == nil:
			data[i], inTree[i] = b, true
		case !errors.Is(err, fs.ErrNotExist):
			return err
		}
	}
	if err := r.checkFastForward(h, paths, data, inTree); err != nil {
		return err
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
	for i, p := range paths {
		abs := filepath.Join(r.dir, filepath.FromSlash(p))
		if inTree[i] {
			err = atomicfile.WriteFile(abs, data[i])
		} else if err = os.Remove(abs); errors.Is(err, fs.ErrNotExist) {
			err = nil
		}
		if err != nil {
			return err
		}
	}
	w, err := r.r.Worktree()
	if err != nil {
		return err
	}
	if err := r.stage(w, paths); err != nil {
		return err
	}
	return r.r.Storer.SetReference(plumbing.NewHashReference(mainRef, h))
}

var mainRef = plumbing.NewBranchReferenceName("main")

// checkFastForward returns ErrWorkTreeChanged unless HEAD is a symbolic
// reference to main, h descends from HEAD, and each path on disk is the same
// as in HEAD or as in h, given by data and inTree (a path missing from both
// is the same). A path already as in h is what a fast-forward that stopped
// before moving the reference leaves; this one finishes it.
func (r *Repo) checkFastForward(h plumbing.Hash, paths []string, data [][]byte, inTree []bool) error {
	ref, err := r.r.Storer.Reference(plumbing.HEAD)
	if err != nil {
		return err
	}
	if ref.Type() != plumbing.SymbolicReference || ref.Target() != mainRef {
		return fmt.Errorf("%w: HEAD is not on main", ErrWorkTreeChanged)
	}
	head, err := r.Head()
	if err != nil {
		return err
	}
	if ok, err := r.IsAncestor(head, h); err != nil || !ok {
		if err != nil {
			return err
		}
		return fmt.Errorf("%w: %s does not descend from HEAD", ErrWorkTreeChanged, h)
	}
	same := func(a []byte, inA bool, b []byte, inB bool) bool {
		return inA == inB && bytes.Equal(a, b)
	}
	for i, p := range paths {
		got, err := atomicfile.ReadFile(filepath.Join(r.dir, filepath.FromSlash(p)))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		onDisk := err == nil
		if same(got, onDisk, data[i], inTree[i]) {
			continue
		}
		old, err := r.ReadAt(head, p)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if !same(got, onDisk, old, err == nil) {
			return fmt.Errorf("%w: %s differs from HEAD", ErrWorkTreeChanged, p)
		}
	}
	return nil
}

// checkWorkPath refuses a path that is not a file of the work tree.
func checkWorkPath(p string) error {
	first, _, _ := strings.Cut(path.Clean(p), "/")
	if !filepath.IsLocal(filepath.FromSlash(p)) || strings.EqualFold(first, ".git") {
		return fmt.Errorf("%q is not a path in the work tree", p)
	}
	return nil
}
