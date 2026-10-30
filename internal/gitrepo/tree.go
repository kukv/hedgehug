package gitrepo

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/utils/merkletrie"
)

// ErrTreeNotFound means a tree the caller named is not in the repository.
var ErrTreeNotFound = errors.New("tree not found")

// Change is a file that differs between two trees.
type Change struct {
	Path    string // relative to the repository root, slash-separated
	Deleted bool
}

// HeadTree returns the tree of the commit HEAD points at.
func (r *Repo) HeadTree() (plumbing.Hash, error) {
	ref, err := r.r.Head()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	c, err := r.r.CommitObject(ref.Hash())
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return c.TreeHash, nil
}

func (r *Repo) tree(h plumbing.Hash) (*object.Tree, error) {
	t, err := r.r.TreeObject(h)
	if errors.Is(err, plumbing.ErrObjectNotFound) {
		return nil, fmt.Errorf("%s: %w", h, ErrTreeNotFound)
	}
	return t, err
}

// DiffTrees returns the files under prefix that were added, changed or
// removed from tree from to tree to. Renames come as a removal and an
// addition.
func (r *Repo) DiffTrees(from, to plumbing.Hash, prefix string) ([]Change, error) {
	a, err := r.tree(from)
	if err != nil {
		return nil, err
	}
	b, err := r.tree(to)
	if err != nil {
		return nil, err
	}
	cs, err := object.DiffTree(a, b)
	if err != nil {
		return nil, err
	}
	var out []Change
	for _, c := range cs {
		action, err := c.Action()
		if err != nil {
			return nil, err
		}
		ch := Change{Path: c.To.Name}
		if action == merkletrie.Delete {
			ch = Change{Path: c.From.Name, Deleted: true}
		}
		if strings.HasPrefix(ch.Path, prefix) {
			out = append(out, ch)
		}
	}
	return out, nil
}

// ReadFile returns the content of path in tree. A missing path is
// fs.ErrNotExist.
func (r *Repo) ReadFile(tree plumbing.Hash, path string) ([]byte, error) {
	t, err := r.tree(tree)
	if err != nil {
		return nil, err
	}
	f, err := t.File(path)
	if errors.Is(err, object.ErrFileNotFound) {
		return nil, fmt.Errorf("%s: %w", path, fs.ErrNotExist)
	}
	if err != nil {
		return nil, err
	}
	rd, err := f.Reader()
	if err != nil {
		return nil, err
	}
	b, err := io.ReadAll(rd)
	if cerr := rd.Close(); err == nil {
		err = cerr
	}
	return b, err
}

// Files returns the paths of the files under prefix in tree. The order is
// not fixed.
func (r *Repo) Files(tree plumbing.Hash, prefix string) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		t, err := r.tree(tree)
		if err != nil {
			yield("", err)
			return
		}
		w := object.NewTreeWalker(t, true, nil)
		defer w.Close()
		for {
			name, e, err := w.Next()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				yield("", err)
				return
			}
			if !e.Mode.IsFile() || !strings.HasPrefix(name, prefix) {
				continue
			}
			if !yield(name, nil) {
				return
			}
		}
	}
}
