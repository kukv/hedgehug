package index

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// maxForwardHops is how many forwards in a row the search for the last
// version of a task as a task goes back through.
const maxForwardHops = 20

// ForwardEntry is what the index keeps of a forward: a task file that moved
// to another store.
type ForwardEntry struct {
	ULID      model.ULID
	MovedTo   model.ULID // the ulid of the store it moved to
	ID        string     // the id the task had when it was last a task here; "" when not found
	FormerIDs []string
	Broken    string // why the forward did not decode; empty when it did
}

// Forwards returns every forward, in ULID order.
func (ix *Index) Forwards() []*ForwardEntry {
	out := slices.Collect(maps.Values(ix.forwards))
	slices.SortFunc(out, func(a, b *ForwardEntry) int { return cmp.Compare(a.ULID.String(), b.ULID.String()) })
	return out
}

// Forward returns the forward of ulid.
func (ix *Index) Forward(ulid model.ULID) (*ForwardEntry, bool) {
	f, ok := ix.forwards[ulid]
	return f, ok
}

// ResolveForward finds the forward s names, ignoring case: by its id, a
// former id, or its full ULID, trying them in that order as Resolve does.
// Only forwards are looked at, and not the broken ones: a file that does not
// decode names no store (doctor tells of it). An input that matches more than one is an
// *AmbiguousError; none is store.ErrNotFound.
func (ix *Index) ResolveForward(s string) (*ForwardEntry, error) {
	if s == "" {
		return nil, fmt.Errorf("%q: %w", s, store.ErrNotFound)
	}
	u := strings.ToUpper(s)
	stages := []func(*ForwardEntry) bool{
		func(f *ForwardEntry) bool { return f.ID != "" && strings.EqualFold(f.ID, s) },
		func(f *ForwardEntry) bool {
			return slices.ContainsFunc(f.FormerIDs, func(k string) bool { return strings.EqualFold(k, s) })
		},
		func(f *ForwardEntry) bool { return len(u) == 26 && f.ULID.String() == u },
	}
	all := slices.DeleteFunc(ix.Forwards(), func(f *ForwardEntry) bool { return f.Broken != "" })
	for _, match := range stages {
		var hits []*ForwardEntry
		for _, f := range all {
			if match(f) {
				hits = append(hits, f)
			}
		}
		switch len(hits) {
		case 0:
			continue
		case 1:
			return hits[0], nil
		}
		ulids := make([]model.ULID, len(hits))
		for i, f := range hits {
			ulids[i] = f.ULID
		}
		return nil, &AmbiguousError{Input: s, Candidates: ulids}
	}
	return nil, fmt.Errorf("%q: %w", s, store.ErrNotFound)
}

// newForwardEntry decodes the forward file data found at the path of ulid.
// The id it had as a task is read from the history of HEAD. A file that
// does not decode, or whose ULID is not the one of its path, gives a broken
// entry.
func (ix *Index) newForwardEntry(ulid model.ULID, data []byte) (*ForwardEntry, error) {
	f, err := model.DecodeForward(data)
	if err == nil && f.ULID != ulid {
		err = fmt.Errorf("ulid %s does not match the file name", f.ULID)
	}
	if err != nil {
		return &ForwardEntry{ULID: ulid, Broken: err.Error()}, nil
	}
	e := &ForwardEntry{ULID: ulid, MovedTo: f.MovedTo}
	t, err := ix.lastTask(ulid)
	if err != nil {
		return nil, err
	}
	if t != nil {
		e.ID, e.FormerIDs = t.ID, t.FormerIDs
	}
	return e, nil
}

// lastTask reads the last version of the file of ulid that was a task, from
// the history of HEAD: it finds the commit that last changed the file and
// reads the file as that commit's first parent had it, going further back
// while that is a forward too. It is nil when there is none, or it does not
// decode.
func (ix *Index) lastTask(ulid model.ULID) (*model.Task, error) {
	p := model.TaskPath(ulid)
	from, err := ix.repo.Head()
	if err != nil {
		return nil, err
	}
	for range maxForwardHops {
		c, ok, err := ix.repo.LastChange(from, plumbing.ZeroHash, p)
		if err != nil {
			return nil, err
		}
		if !ok || len(c.Parents) == 0 {
			return nil, nil
		}
		before := c.Parents[0]
		b, err := ix.repo.ReadAt(before, p)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if model.IsForward(b) {
			from = before
			continue
		}
		t, err := model.DecodeTask(b)
		if err != nil {
			return nil, nil //nolint:nilerr // a version that does not decode gives no id
		}
		return t, nil
	}
	return nil, nil
}
