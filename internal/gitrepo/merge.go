package gitrepo

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func (r *Repo) commit(h plumbing.Hash) (*object.Commit, error) {
	c, err := r.r.CommitObject(h)
	if err != nil {
		return nil, fmt.Errorf("commit %s: %w", h, err)
	}
	return c, nil
}

// MergeBase returns a newest common ancestor of a and b, or the zero hash
// when they have none or either is zero.
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
	return bases[0].Hash, nil
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
// every parent, oldest first. A zero base excludes nothing.
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

// LastChange returns the latest commit between base and from, as Between
// counts them, that changed path: one whose content of path differs from its
// parent's, or, for a merge, from every parent's, so that a merge that takes
// one side's content is not a change of its own. ok is false when no commit
// changed it.
func (r *Repo) LastChange(from, base plumbing.Hash, path string) (e LogEntry, ok bool, err error) {
	cs, err := r.side(from, base)
	if err != nil {
		return LogEntry{}, false, err
	}
	for _, c := range slices.Backward(cs) {
		changed, err := r.changes(c, path)
		if err != nil {
			return LogEntry{}, false, err
		}
		if changed {
			return entry(c), true, nil
		}
	}
	return LogEntry{}, false, nil
}

// side returns the commits reachable from from but not from base, oldest
// first. The order is by author time; commits of the same second, the
// precision git keeps, come ancestors first and then by hash, so that the
// order is fixed whatever order the parents are in.
func (r *Repo) side(from, base plumbing.Hash) ([]*object.Commit, error) {
	if from.IsZero() {
		return nil, nil
	}
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
	// depth is the longest path to a commit of the side without parents in it.
	depth := map[plumbing.Hash]int{}
	var depthOf func(c *object.Commit) int
	depthOf = func(c *object.Commit) int {
		if d, ok := depth[c.Hash]; ok {
			return d
		}
		d := 0
		for _, p := range c.ParentHashes {
			if pc, ok := in[p]; ok {
				d = max(d, depthOf(pc)+1)
			}
		}
		depth[c.Hash] = d
		return d
	}
	out := make([]*object.Commit, 0, len(in))
	for _, c := range in {
		depthOf(c)
		out = append(out, c)
	}
	slices.SortFunc(out, func(a, b *object.Commit) int {
		return cmp.Or(
			a.Author.When.Compare(b.Author.When),
			cmp.Compare(depth[a.Hash], depth[b.Hash]),
			strings.Compare(a.Hash.String(), b.Hash.String()),
		)
	})
	return out, nil
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
// not is removed. Nothing else in the work tree is touched. Every path is
// read from h before anything is written, so a commit or object that is
// missing changes nothing.
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
			err = atomicWrite(abs, data[i])
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
	return r.r.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), h))
}

// checkWorkPath refuses a path that is not a file of the work tree.
func checkWorkPath(p string) error {
	first, _, _ := strings.Cut(path.Clean(p), "/")
	if !filepath.IsLocal(filepath.FromSlash(p)) || strings.EqualFold(first, ".git") {
		return fmt.Errorf("%q is not a path in the work tree", p)
	}
	return nil
}

// atomicWrite replaces the file at path with data through a temporary file in
// the same directory, so that a reader sees either the old or the new content.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
