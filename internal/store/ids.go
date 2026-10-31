package store

import (
	"errors"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/atomicfile"
	"github.com/kukv/hedgehug/internal/model"
)

// IDNumbers returns, for each id prefix, the largest number used by the
// ids and former ids of the tasks as HEAD has them. Update calls it under
// the lock, so HEAD does not move while it counts.
type IDNumbers func() (map[string]int, error)

// SetIDNumbers makes Update count numbers with f instead of reading every
// task file. The index sets it.
func (s *Store) SetIDNumbers(f IDNumbers) { s.idNumbers = f }

// usedNumbers counts the numbers in use: the tasks' ids and former ids,
// the record of purged numbers, and the ids given in this operation.
func (tx *Tx) usedNumbers() (map[string]int, error) {
	var out map[string]int
	if f := tx.s.idNumbers; f != nil {
		m, err := f()
		if err != nil {
			return nil, err
		}
		out = m
	} else {
		out = map[string]int{}
		for t, err := range tx.s.Tasks() {
			var fe *FileError
			if errors.As(err, &fe) {
				// Its ids cannot be read, but a number it holds must not
				// be given again.
				b, err := atomicfile.ReadFile(tx.s.path(fe.Path))
				if err != nil {
					return nil, err
				}
				raise(out, model.IDNumbersIn(b))
				continue
			}
			if err != nil {
				return nil, err
			}
			countIDs(out, t)
		}
	}
	if tx.numbers == nil {
		n, err := tx.s.Numbers()
		if err != nil {
			return nil, err
		}
		tx.numbers = n
	}
	raise(out, tx.numbers)
	for _, t := range tx.tasks {
		if t != nil {
			countIDs(out, t)
		}
	}
	return out, nil
}

// raise raises used to the numbers of more.
func raise(used, more map[string]int) {
	for p, n := range more {
		if n > used[p] {
			used[p] = n
		}
	}
}

// countIDs raises used to the numbers of t's id and former ids.
func countIDs(used map[string]int, t *model.Task) {
	for _, k := range append([]string{t.ID}, t.FormerIDs...) {
		if p, n, ok := model.ParseTaskID(k); ok && n > used[p] {
			used[p] = n
		}
	}
}

// assignIDs gives an id to each task put in the operation that has none,
// or whose id's prefix is not its workspace's. The old id goes to the
// former ids. Tasks without a workspace that exists are left to validate.
func (tx *Tx) assignIDs() error {
	var used map[string]int
	for _, ulid := range tx.taskOrder {
		t := tx.tasks[ulid]
		if t == nil || t.Workspace.IsZero() {
			continue
		}
		ws, err := tx.workspace(t.Workspace)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if err := model.ValidIDPrefix(ws.IDPrefix); err != nil {
			return fmt.Errorf("%s: %w; fix it with hedgehug workspace prefix", model.WorkspacePath(ws.ULID), err)
		}
		if p, _, ok := model.ParseTaskID(t.ID); ok && p == ws.IDPrefix {
			continue
		}
		if used == nil {
			if used, err = tx.usedNumbers(); err != nil {
				return err
			}
		}
		used[ws.IDPrefix]++
		tx.oldIDs = append(tx.oldIDs, oldID{t, t.ID, slices.Clone(t.FormerIDs)})
		if t.ID != "" && !slices.Contains(t.FormerIDs, t.ID) {
			t.FormerIDs = append(t.FormerIDs, t.ID)
		}
		t.ID = model.FormatTaskID(ws.IDPrefix, used[ws.IDPrefix])
	}
	return nil
}

// oldID is the id and former ids a task had before assignIDs.
type oldID struct {
	t      *model.Task
	id     string
	former []string
}

// restoreIDs puts back the ids assignIDs and the ranks rerank changed.
func (tx *Tx) restoreIDs() {
	for _, o := range tx.oldRanks {
		o.t.Rank = o.rank
	}
	for _, o := range tx.oldIDs {
		o.t.ID, o.t.FormerIDs = o.id, o.former
	}
}
