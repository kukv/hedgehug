package mount

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// Hit is a task found in a store.
type Hit struct {
	Store *Mounted
	Entry *index.Entry
}

// Ref is the task as "<store>:<id>", or "<store>:<ULID>" when it has no id.
func (h Hit) Ref() string {
	id := h.Entry.ID
	if id == "" {
		id = h.Entry.ULID.String()
	}
	return h.Store.Name + ":" + id
}

// AmbiguousError is an input that matches tasks in more than one store.
// Candidates are "<store>:<id>" (the ULID when the task has no id), in the
// order of the stores.
type AmbiguousError struct {
	Input      string
	Candidates []Hit
}

func (e *AmbiguousError) Error() string {
	lines := make([]string, len(e.Candidates))
	for i, c := range e.Candidates {
		lines[i] = fmt.Sprintf("%s (%s) %s", c.Ref(), c.Entry.ULID, c.Entry.Title)
	}
	stores := map[*Mounted]bool{}
	for _, c := range e.Candidates {
		stores[c.Store] = true
	}
	where := fmt.Sprintf("%d stores", len(stores))
	if len(stores) == 1 {
		where = "1 store"
	}
	return fmt.Sprintf("%q matches %d tasks in %s:\n  %s", e.Input, len(e.Candidates), where, strings.Join(lines, "\n  "))
}

// Resolve finds the task arg names in every store, or in the store its
// "<store>:" names. A match in one store is that task; none is
// store.ErrNotFound; in several stores is *AmbiguousError. An input that
// matches more than one task within one store is that index's
// *index.AmbiguousError when the set has one store only. With two or more it
// is an *AmbiguousError too, with the candidates of that store and the hits
// of the other stores together, so that the answer does not depend on which
// store comes first.
func (s *Set) Resolve(arg string) (Hit, error) {
	stores, in, named := s.Stores, arg, false
	if m, rest, ok := s.SplitStore(arg); ok {
		stores, in, named = []*Mounted{m}, rest, true
	}
	var hits []Hit
	for _, m := range stores {
		var found []*index.Entry
		var rerr error
		err := m.With(func(ix *index.Index) {
			ulid, err := ix.Resolve(in)
			var ie *index.AmbiguousError
			ulids := []model.ULID{ulid}
			if errors.As(err, &ie) && len(s.Stores) > 1 {
				ulids, err = ie.Candidates, nil
			}
			if rerr = err; rerr != nil {
				return
			}
			for _, u := range ulids {
				e, ok := ix.Task(u)
				if !ok {
					rerr = fmt.Errorf("%s: the index resolved %s but has no such task", m.Name, u)
					return
				}
				found = append(found, e)
			}
		})
		if err != nil {
			if named {
				return Hit{}, fmt.Errorf("cannot open %s: %w", m.Name, err)
			}
			// treated like a store that did not open: say so, look in the others
			if s.unopened != nil {
				s.unopened(Skipped{Name: m.Name, Err: err})
			}
			continue
		}
		if errors.Is(rerr, store.ErrNotFound) {
			continue
		}
		if rerr != nil {
			return Hit{}, rerr
		}
		for _, e := range found {
			hits = append(hits, Hit{Store: m, Entry: e})
		}
	}
	switch len(hits) {
	case 0:
		return Hit{}, fmt.Errorf("%q: %w", arg, store.ErrNotFound)
	case 1:
		return hits[0], nil
	}
	return Hit{}, &AmbiguousError{Input: arg, Candidates: hits}
}
