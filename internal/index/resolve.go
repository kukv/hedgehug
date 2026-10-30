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
	Candidates []model.ID // in ID order
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("%q matches %d tasks", e.Input, len(e.Candidates))
}

// Resolve finds the task s names, ignoring case. It tries in order a key, a
// former key, and a full ULID, and returns what the first stage that matches
// finds. Deleted tasks count.
func (ix *Index) Resolve(s string) (model.ID, error) {
	if s == "" {
		return model.ID{}, fmt.Errorf("%q: %w", s, store.ErrNotFound)
	}
	u := strings.ToUpper(s)
	stages := []func(*Entry) bool{
		func(e *Entry) bool { return e.Key != "" && strings.EqualFold(e.Key, s) },
		func(e *Entry) bool {
			return slices.ContainsFunc(e.FormerKeys, func(k string) bool { return strings.EqualFold(k, s) })
		},
		func(e *Entry) bool { return len(u) == 26 && e.ID.String() == u },
	}
	for _, match := range stages {
		var hits []model.ID
		for _, e := range ix.tasks {
			if match(e) {
				hits = append(hits, e.ID)
			}
		}
		switch len(hits) {
		case 0:
			continue
		case 1:
			return hits[0], nil
		}
		slices.SortFunc(hits, func(a, b model.ID) int { return cmp.Compare(a.String(), b.String()) })
		return model.ID{}, &AmbiguousError{Input: s, Candidates: hits}
	}
	return model.ID{}, fmt.Errorf("%q: %w", s, store.ErrNotFound)
}

// KeyNumbers returns, for each key prefix, the largest number of the keys
// and former keys of the tasks at HEAD, deleted ones included, and of the
// BrokenNumbers of the files that do not decode. It is the
// store's KeyNumbers: Update calls it under the lock. Every call returns a
// new map, since the store changes the one it gets.
func (ix *Index) KeyNumbers() (map[string]int, error) {
	if err := ix.Refresh(); err != nil {
		return nil, err
	}
	out := map[string]int{}
	for _, e := range ix.tasks {
		for _, k := range append([]string{e.Key}, e.FormerKeys...) {
			if p, n, ok := model.ParseKey(k); ok && n > out[p] {
				out[p] = n
			}
		}
		for p, n := range e.BrokenNumbers {
			if n > out[p] {
				out[p] = n
			}
		}
	}
	return out, nil
}
