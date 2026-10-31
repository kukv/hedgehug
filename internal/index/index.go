// Package index keeps an index of the tasks of a store, for queries and for
// finding a task by its id. It reads task files from the tree of HEAD, so a
// change that is not committed is not seen.
package index

import (
	"errors"
	"maps"
	"slices"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

// Index is the index of one store, as of one tree of HEAD.
type Index struct {
	s     *store.Store
	repo  *gitrepo.Repo
	tree  plumbing.Hash
	sc    *schema.Schema // schema.yaml as tree has it
	tasks map[model.ULID]*Entry

	// Read from the tree each time it changes; the cache does not hold them.
	workspaces map[model.ULID]*model.Workspace

	// Derived from tasks by derive.
	children map[model.ULID][]*Entry
	roots    []*Entry
	problems map[model.ULID][]Problem
}

// Open opens the index of s. It reads the cache in .hedgehug/local and
// brings it to the tree of HEAD, reading only the task files that changed;
// with no usable cache it reads every task file. It writes the cache back.
func Open(s *store.Store) (*Index, error) {
	ix := &Index{s: s, repo: s.Repo()}
	head, err := ix.repo.HeadTree()
	if err != nil {
		return nil, err
	}
	// The problems depend on the workspaces, so read them first.
	if err := ix.readWorkspaces(head); err != nil {
		return nil, err
	}
	tree, ok := ix.load()
	rebuilt := !ok
	if ok {
		err = ix.update(tree, head)
	}
	if !ok || errors.Is(err, gitrepo.ErrTreeNotFound) {
		err = ix.rebuild(head)
		rebuilt = true
	}
	if err != nil {
		return nil, err
	}
	if rebuilt || tree != head {
		ix.save()
	}
	s.SetIDNumbers(ix.IDNumbers)
	s.SetSiblings(ix.siblingULIDs)
	return ix, nil
}

// Refresh brings the index to the tree of HEAD when HEAD has moved, as after
// a write or a sync. After an error, Open the index again.
func (ix *Index) Refresh() error {
	head, err := ix.repo.HeadTree()
	if err != nil {
		return err
	}
	if head == ix.tree {
		return nil
	}
	if err := ix.readWorkspaces(head); err != nil {
		return err
	}
	err = ix.update(ix.tree, head)
	if errors.Is(err, gitrepo.ErrTreeNotFound) {
		err = ix.rebuild(head)
	}
	if err != nil {
		return err
	}
	ix.save()
	return nil
}

// Schema returns schema.yaml as the indexed tree has it.
func (ix *Index) Schema() *schema.Schema { return ix.sc }

// Task returns the entry of ulid. Deleted tasks are in the index too.
func (ix *Index) Task(ulid model.ULID) (*Entry, bool) {
	e, ok := ix.tasks[ulid]
	return e, ok
}

// Tasks returns every entry, deleted ones included. The order is not fixed.
func (ix *Index) Tasks() []*Entry { return slices.Collect(maps.Values(ix.tasks)) }

// Children returns the tasks whose parent is ulid, by rank, ID, then ULID.
func (ix *Index) Children(ulid model.ULID) []*Entry { return slices.Clone(ix.children[ulid]) }

// Roots returns the tasks with no parent, and those whose parent is not in
// the index, by rank, ID, then ULID.
func (ix *Index) Roots() []*Entry { return slices.Clone(ix.roots) }

// derive fills what is computed from the entries. Call it after any change
// to tasks.
func (ix *Index) derive() {
	ix.children = map[model.ULID][]*Entry{}
	ix.roots = nil
	for _, e := range ix.tasks {
		if e.Parent != nil {
			ix.children[*e.Parent] = append(ix.children[*e.Parent], e)
		}
		if e.Parent == nil || ix.tasks[*e.Parent] == nil {
			ix.roots = append(ix.roots, e)
		}
	}
	for _, cs := range ix.children {
		sortByRank(cs)
	}
	sortByRank(ix.roots)
}

// rankKey is what orders e among its siblings.
func rankKey(e *Entry) model.RankKey {
	return model.RankKey{ID: e.ID, ULID: e.ULID, Rank: e.Rank}
}

func sortByRank(es []*Entry) {
	slices.SortFunc(es, func(a, b *Entry) int {
		return model.CompareRankKeys(rankKey(a), rankKey(b))
	})
}
