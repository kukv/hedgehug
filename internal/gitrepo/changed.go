package gitrepo

import (
	"maps"
	"slices"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Entry reads the commit h as a LogEntry.
func (r *Repo) Entry(h plumbing.Hash) (LogEntry, error) {
	c, err := r.commit(h)
	if err != nil {
		return LogEntry{}, err
	}
	return entry(c), nil
}

// ChangedFiles returns the commit h and the files it changed, sorted: for a
// commit with one parent, those that differ from the parent; for a merge,
// those that differ from every parent, so that a merge that takes one
// side's content changed nothing of its own (as LastChange counts); for a
// root commit, every file. Only the trees whose hashes differ are read.
func (r *Repo) ChangedFiles(h plumbing.Hash) (LogEntry, []string, error) {
	c, err := r.commit(h)
	if err != nil {
		return LogEntry{}, nil, err
	}
	t, err := r.tree(c.TreeHash)
	if err != nil {
		return LogEntry{}, nil, err
	}
	if c.NumParents() == 0 {
		var out []string
		if err := r.diffTrees(nil, t, "", func(p string, _ bool) { out = append(out, p) }); err != nil {
			return LogEntry{}, nil, err
		}
		slices.Sort(out)
		return entry(c), out, nil
	}
	count := map[string]int{}
	for _, ph := range c.ParentHashes {
		p, err := r.commit(ph)
		if err != nil {
			return LogEntry{}, nil, err
		}
		pt, err := r.tree(p.TreeHash)
		if err != nil {
			return LogEntry{}, nil, err
		}
		if err := r.diffTrees(pt, t, "", func(path string, _ bool) { count[path]++ }); err != nil {
			return LogEntry{}, nil, err
		}
	}
	var out []string
	for p, n := range count {
		if n == c.NumParents() {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return entry(c), out, nil
}

// diffTrees calls fn with every file path that differs between a and b
// (either may be nil, an empty tree), going down only into directories
// whose hashes differ. deleted is true for a file that a has and b does
// not. A path that is not a directory is a file here, a submodule (which a
// store does not have) too, as git log -- path counts it. The entries of a
// directory come sorted by their bare names (directory "a" before file
// "a.md"), as go-git's object.DiffTree lists them.
func (r *Repo) diffTrees(a, b *object.Tree, dir string, fn func(path string, deleted bool)) error {
	byName := func(t *object.Tree) map[string]object.TreeEntry {
		m := map[string]object.TreeEntry{}
		if t != nil {
			for _, e := range t.Entries {
				m[e.Name] = e
			}
		}
		return m
	}
	am, bm := byName(a), byName(b)
	names := map[string]bool{}
	for n := range am {
		names[n] = true
	}
	for n := range bm {
		names[n] = true
	}
	for _, n := range slices.Sorted(maps.Keys(names)) {
		ea, inA := am[n]
		eb, inB := bm[n]
		if inA && inB && ea.Hash == eb.Hash && ea.Mode == eb.Mode {
			continue
		}
		var sa, sb *object.Tree
		var err error
		if inA && ea.Mode == filemode.Dir {
			if sa, err = r.tree(ea.Hash); err != nil {
				return err
			}
		} else if inA {
			fn(dir+n, !inB || eb.Mode == filemode.Dir)
		}
		if inB && eb.Mode == filemode.Dir {
			if sb, err = r.tree(eb.Hash); err != nil {
				return err
			}
		}
		if sa != nil || sb != nil {
			if err := r.diffTrees(sa, sb, dir+n+"/", fn); err != nil {
				return err
			}
		}
		// After the contents of a directory that became this file, as
		// go-git lists them.
		if inB && eb.Mode != filemode.Dir && (!inA || ea.Mode == filemode.Dir) {
			fn(dir+n, false)
		}
	}
	return nil
}
