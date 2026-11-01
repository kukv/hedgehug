package mount

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// CrossRef is a reference of a task to a task that is not in its store
// but in another mounted store.
type CrossRef struct {
	From   Entry  // the task that refers
	Field  string // "parent", "blocked_by" or the task_ref property
	Target Entry  // the task in the other store
}

// pending is a reference that its own store does not hold.
type pending struct {
	from  *index.Entry
	field string
	ulid  model.ULID
}

// CrossRefs finds the cross-store references of the live tasks of the
// store name (all stores when name is ""), against every mounted store. A
// reference that its own store holds, even to a deleted task, is not one; a
// target that is deleted in the other store, or in none, is not found. A
// target in more than one other store is reported for the first of them. The
// order is that of the stores, then the ULID of the task, then parent,
// blocked_by and the task_ref properties as the file has them. The data is
// not changed. What it returns belongs to the indexes of the stores, as
// what Query returns does: read it with Mounted.With.
func CrossRefs(set *Set, name string) ([]CrossRef, error) {
	sources := set.Stores
	if name != "" {
		m, ok := set.Get(name)
		if !ok {
			return nil, nil // a store that did not open has no index to read
		}
		sources = []*Mounted{m}
	}
	var out []CrossRef
	for _, m := range sources {
		// Not inside the With of m: With of another store would lock two
		// indexes at once, in an order that two callers could reverse.
		var open []pending
		if err := m.With(func(ix *index.Index) { open = unresolved(ix) }); err != nil {
			return nil, fmt.Errorf("cannot open %s: %w", m.Name, err)
		}
		for _, p := range open {
			t, err := set.liveElsewhere(m, p.ulid)
			if err != nil {
				return nil, err
			}
			if t != nil {
				out = append(out, CrossRef{From: Entry{Store: m, Entry: p.from}, Field: p.field, Target: *t})
			}
		}
	}
	return out, nil
}

// unresolved lists the references of the live tasks of ix that point at no
// task of ix.
func unresolved(ix *index.Index) []pending {
	tasks := ix.Tasks()
	slices.SortFunc(tasks, func(a, b *index.Entry) int { return strings.Compare(a.ULID.String(), b.ULID.String()) })
	var out []pending
	for _, e := range tasks {
		if e.DeletedAt != nil {
			continue
		}
		add := func(field string, ulid model.ULID) {
			if _, ok := ix.Task(ulid); !ok {
				out = append(out, pending{from: e, field: field, ulid: ulid})
			}
		}
		if e.Parent != nil {
			add("parent", *e.Parent)
		}
		for _, u := range e.BlockedBy {
			add("blocked_by", u)
		}
		for _, p := range e.Props {
			if sp, ok := ix.Schema().Prop(p.Key); !ok || sp.Type != schema.TypeTaskRef {
				continue
			}
			for _, u := range e.Targets(p.Key) {
				add(p.Key, u)
			}
		}
	}
	return out
}

// liveElsewhere finds the task ulid, not deleted, in the first mounted store
// other than not; nil when there is none.
func (s *Set) liveElsewhere(not *Mounted, ulid model.ULID) (*Entry, error) {
	for _, m := range s.Stores {
		if m == not {
			continue
		}
		var found *index.Entry
		err := m.With(func(ix *index.Index) {
			if e, ok := ix.Task(ulid); ok && e.DeletedAt == nil {
				found = e
			}
		})
		if err != nil {
			return nil, fmt.Errorf("cannot open %s: %w", m.Name, err)
		}
		if found != nil {
			return &Entry{Store: m, Entry: found}, nil
		}
	}
	return nil, nil
}
