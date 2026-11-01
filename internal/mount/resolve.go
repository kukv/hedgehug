package mount

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/index"
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
	stores, in := s.Stores, arg
	if m, rest, ok := s.SplitStore(arg); ok {
		stores, in = []*Mounted{m}, rest
	}
	var hits []Hit
	for _, m := range stores {
		ix, err := m.Index()
		if err != nil {
			return Hit{}, fmt.Errorf("%s: %w", m.Name, err)
		}
		ulid, err := ix.Resolve(in)
		if errors.Is(err, store.ErrNotFound) {
			continue
		}
		if err != nil {
			return Hit{}, err
		}
		if e, ok := ix.Task(ulid); ok {
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
