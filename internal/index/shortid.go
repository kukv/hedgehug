package index

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// minShort is the shortest short ID.
const minShort = 4

// ShortID returns the last characters of id that tell it apart from every
// other task of the store, deleted ones included: at least minShort. The
// end of a ULID is random, while its start is a timestamp that tasks made
// together share. A short ID can grow as tasks are added, so history keeps
// full IDs.
func (ix *Index) ShortID(id model.ID) string {
	s := id.String()
	n, ok := ix.short[id]
	if !ok {
		n = minShort
	}
	return s[len(s)-n:]
}

// ShortLengths returns the length of the short ID of each of ids, by the
// rule of ShortID: at least minShort characters of the end, and as many more
// as it takes to differ from the others. Sorting the reversed IDs puts the
// IDs that share the longest ending next to each other, so each needs
// comparing with its neighbors only.
func ShortLengths(ids []model.ID) map[model.ID]int {
	type item struct {
		rev string
		id  model.ID
	}
	items := make([]item, 0, len(ids))
	for _, id := range ids {
		items = append(items, item{reverse(id.String()), id})
	}
	slices.SortFunc(items, func(a, b item) int { return cmp.Compare(a.rev, b.rev) })
	out := make(map[model.ID]int, len(items))
	for i, it := range items {
		n := minShort
		for _, j := range []int{i - 1, i + 1} {
			if j >= 0 && j < len(items) {
				n = max(n, commonPrefix(it.rev, items[j].rev)+1)
			}
		}
		out[it.id] = min(n, len(it.rev))
	}
	return out
}

func reverse(s string) string {
	b := []byte(s) // IDs are ASCII
	slices.Reverse(b)
	return string(b)
}

func commonPrefix(a, b string) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}

// AmbiguousError means an input matches more than one task.
type AmbiguousError struct {
	Input      string
	Candidates []model.ID // in ID order
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("%q matches %d tasks", e.Input, len(e.Candidates))
}

// Resolve finds the task s names, ignoring case. It tries in order a key, a
// former key, a full ULID, and the end of a ULID of at least minShort
// characters, and returns what the first stage that matches finds. Deleted
// tasks count.
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
		func(e *Entry) bool { return len(u) >= minShort && strings.HasSuffix(e.ID.String(), u) },
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
