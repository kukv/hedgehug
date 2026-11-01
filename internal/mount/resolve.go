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
	return fmt.Sprintf("%q matches %d tasks in %d stores:\n  %s", e.Input, len(e.Candidates), len(e.Candidates), strings.Join(lines, "\n  "))
}

// Resolve finds the task arg names in every store, or in the store its
// "<store>:" names. A match in one store is that task; none is
// store.ErrNotFound; in several stores is *AmbiguousError. An input that
// matches more than one task within one store is that index's
// *index.AmbiguousError, as now.
func (s *Set) Resolve(arg string) (Hit, error) {
	stores, in, named := s.Stores, arg, false
	if m, rest, ok := s.SplitStore(arg); ok {
		stores, in, named = []*Mounted{m}, rest, true
	}
	var hits []Hit
	for _, m := range stores {
		var ulid model.ULID
		var e *index.Entry
		var rerr error
		err := m.With(func(ix *index.Index) {
			if ulid, rerr = ix.Resolve(in); rerr != nil {
				return
			}
			var ok bool
			if e, ok = ix.Task(ulid); !ok {
				rerr = fmt.Errorf("%s: the index resolved %s but has no such task", m.Name, ulid)
			}
		})
		if err != nil {
			if named {
				return Hit{}, fmt.Errorf("cannot open %s: %w", m.Name, err)
			}
			// treated like a store that did not open: warn, look in the others
			s.warnOnce(m.Name, fmt.Sprintf("cannot open %s: %v", m.Name, err))
			continue
		}
		if errors.Is(rerr, store.ErrNotFound) {
			continue
		}
		if rerr != nil {
			return Hit{}, rerr
		}
		hits = append(hits, Hit{Store: m, Entry: e})
	}
	switch len(hits) {
	case 0:
		return Hit{}, fmt.Errorf("%q: %w", arg, store.ErrNotFound)
	case 1:
		return hits[0], nil
	}
	return Hit{}, &AmbiguousError{Input: arg, Candidates: hits}
}

// warnOnce gives msg to Options.Warn, the first time for the store name.
func (s *Set) warnOnce(name, msg string) {
	s.warnedMu.Lock()
	defer s.warnedMu.Unlock()
	if s.warn == nil || s.warned[name] {
		return
	}
	if s.warned == nil {
		s.warned = map[string]bool{}
	}
	s.warned[name] = true
	s.warn(msg)
}
