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
		if me := s.findForward(stores, arg, in); me != nil {
			return Hit{}, me
		}
		return Hit{}, fmt.Errorf("%q: %w", arg, store.ErrNotFound)
	case 1:
		return hits[0], nil
	}
	return Hit{}, &AmbiguousError{Input: arg, Candidates: hits}
}

// MovedError is an input that names only a forward: the task moved to
// another store. To is the store it moved to when it is mounted (nil when
// not), and Entry the task there when found.
type MovedError struct {
	Input   string
	From    *Mounted
	Forward *index.ForwardEntry
	To      *Mounted
	Entry   *index.Entry
}

// Label is the task as the store it left names it: its id, or the input
// when the forward does not remember one.
func (e *MovedError) Label() string {
	if e.Forward.ID != "" {
		return e.Forward.ID
	}
	return e.Input
}

// NowID is the id the task has in the store it moved to; "" when not known.
func (e *MovedError) NowID() string {
	if e.Entry == nil {
		return ""
	}
	return e.Entry.ID
}

func (e *MovedError) Error() string {
	if e.To == nil {
		return fmt.Sprintf("%s moved to another store (ULID %s); mount that store to follow it", e.Label(), e.Forward.MovedTo)
	}
	if e.Entry == nil {
		return fmt.Sprintf("%s moved to %s", e.Label(), e.To.Name)
	}
	return fmt.Sprintf("%s moved to %s (%s)", e.Label(), e.To.Name, Hit{Store: e.To, Entry: e.Entry}.Ref())
}

// Unwrap lets a caller that treats a task that is not here as absent do so
// for a forward.
func (e *MovedError) Unwrap() error { return store.ErrNotFound }

// Moved looks for the task a forward of from points at, in the mounted
// store whose ulid is f.MovedTo. A nil set has no such store.
func (s *Set) Moved(from *Mounted, input string, f *index.ForwardEntry) *MovedError {
	e := &MovedError{Input: input, From: from, Forward: f}
	if s == nil {
		return e
	}
	for _, m := range s.Stores {
		if m.ULID != f.MovedTo {
			continue
		}
		e.To = m
		_ = m.With(func(ix *index.Index) { e.Entry, _ = ix.Task(f.ULID) })
		break
	}
	return e
}

// findForward looks for a forward that in names in stores; the first store
// that has one gives it. A store that does not open, and an input that
// matches several forwards of a store, are left out.
func (s *Set) findForward(stores []*Mounted, input, in string) *MovedError {
	for _, m := range stores {
		var f *index.ForwardEntry
		if m.With(func(ix *index.Index) { f, _ = ix.ResolveForward(in) }) != nil || f == nil {
			continue
		}
		return s.Moved(m, input, f)
	}
	return nil
}
