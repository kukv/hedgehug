package index

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// AmbiguousError means an input matches more than one task.
type AmbiguousError struct {
	Input      string
	Candidates []model.ULID // in ULID order
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("%q matches %d tasks", e.Input, len(e.Candidates))
}

// Resolve finds the task s names, ignoring case. It tries in order an id, a
// former id, and a full ULID, and returns what the first stage that matches
// finds. Deleted tasks count.
func (ix *Index) Resolve(s string) (model.ULID, error) {
	if s == "" {
		return model.ULID{}, fmt.Errorf("%q: %w", s, store.ErrNotFound)
	}
	u := strings.ToUpper(s)
	stages := []func(*Entry) bool{
		func(e *Entry) bool { return e.ID != "" && strings.EqualFold(e.ID, s) },
		func(e *Entry) bool {
			return slices.ContainsFunc(e.FormerIDs, func(k string) bool { return strings.EqualFold(k, s) })
		},
		func(e *Entry) bool { return len(u) == 26 && e.ULID.String() == u },
	}
	for _, match := range stages {
		var hits []model.ULID
		for _, e := range ix.tasks {
			if match(e) {
				hits = append(hits, e.ULID)
			}
		}
		switch len(hits) {
		case 0:
			continue
		case 1:
			return hits[0], nil
		}
		slices.SortFunc(hits, func(a, b model.ULID) int { return cmp.Compare(a.String(), b.String()) })
		return model.ULID{}, &AmbiguousError{Input: s, Candidates: hits}
	}
	return model.ULID{}, fmt.Errorf("%q: %w", s, store.ErrNotFound)
}

// IDNumbers returns, for each id prefix, the largest number of the ids
// and former ids of the tasks at HEAD, deleted ones included, and of the
// BrokenNumbers of the files that do not decode. It is the
// store's IDNumbers: Update calls it under the lock. Every call returns a
// new map, since the store changes the one it gets.
func (ix *Index) IDNumbers() (map[string]int, error) {
	if err := ix.Refresh(); err != nil {
		return nil, err
	}
	out := map[string]int{}
	for _, e := range ix.tasks {
		for _, k := range append([]string{e.ID}, e.FormerIDs...) {
			if p, n, ok := model.ParseTaskID(k); ok && n > out[p] {
				out[p] = n
			}
		}
		for p, n := range e.BrokenNumbers {
			if n > out[p] {
				out[p] = n
			}
		}
	}
	// A forward keeps the ids of the task it was, so that they are not used
	// again.
	for _, f := range ix.forwards {
		for _, k := range append([]string{f.ID}, f.FormerIDs...) {
			if p, n, ok := model.ParseTaskID(k); ok && n > out[p] {
				out[p] = n
			}
		}
	}
	return out, nil
}

// siblingULIDs returns the tasks at HEAD under parent in workspace ws,
// deleted ones included. The top level (nil parent) holds the tasks whose
// parent is not in the index too, as Roots does. A task whose parent is
// given is returned even if that parent is missing, since the operation may
// put the parent. It is the store's Siblings.
func (ix *Index) siblingULIDs(parent *model.ULID, ws model.ULID) ([]model.ULID, error) {
	if err := ix.Refresh(); err != nil {
		return nil, err
	}
	var out []model.ULID
	for _, e := range ix.tasks {
		if e.Workspace != ws {
			continue
		}
		var match bool
		if parent == nil {
			match = e.Parent == nil || ix.tasks[*e.Parent] == nil
		} else {
			match = e.Parent != nil && *e.Parent == *parent
		}
		if match {
			out = append(out, e.ULID)
		}
	}
	return out, nil
}
