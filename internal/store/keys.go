package store

import (
	"errors"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/model"
)

// KeyNumbers returns, for each key prefix, the largest number used by the
// keys and former keys of the tasks as HEAD has them. Update calls it under
// the lock, so HEAD does not move while it counts.
type KeyNumbers func() (map[string]int, error)

// SetKeyNumbers makes Update count numbers with f instead of reading every
// task file. The index sets it.
func (s *Store) SetKeyNumbers(f KeyNumbers) { s.keyNumbers = f }

// usedNumbers counts the numbers in use: the tasks' keys and former keys,
// the record of purged numbers, and the keys given in this operation.
func (tx *Tx) usedNumbers() (map[string]int, error) {
	var out map[string]int
	if f := tx.s.keyNumbers; f != nil {
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
				// Its keys cannot be read, but a number it holds must not
				// be given again.
				b, err := readFile(tx.s.path(fe.Path))
				if err != nil {
					return nil, err
				}
				raise(out, model.KeyNumbersIn(b))
				continue
			}
			if err != nil {
				return nil, err
			}
			countKeys(out, t)
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
			countKeys(out, t)
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

// countKeys raises used to the numbers of t's key and former keys.
func countKeys(used map[string]int, t *model.Task) {
	for _, k := range append([]string{t.Key}, t.FormerKeys...) {
		if p, n, ok := model.ParseKey(k); ok && n > used[p] {
			used[p] = n
		}
	}
}

// assignKeys gives a key to each task put in the operation that has none,
// or whose key's prefix is not its workspace's. The old key goes to the
// former keys. Tasks without a workspace that exists are left to validate.
func (tx *Tx) assignKeys() error {
	var used map[string]int
	for _, id := range tx.taskOrder {
		t := tx.tasks[id]
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
		if err := model.ValidKeyPrefix(ws.KeyPrefix); err != nil {
			return fmt.Errorf("%s: %w; fix it with hedgehug workspace prefix", model.WorkspacePath(ws.ID), err)
		}
		if p, _, ok := model.ParseKey(t.Key); ok && p == ws.KeyPrefix {
			continue
		}
		if used == nil {
			if used, err = tx.usedNumbers(); err != nil {
				return err
			}
		}
		used[ws.KeyPrefix]++
		tx.oldKeys = append(tx.oldKeys, oldKey{t, t.Key, slices.Clone(t.FormerKeys)})
		if t.Key != "" && !slices.Contains(t.FormerKeys, t.Key) {
			t.FormerKeys = append(t.FormerKeys, t.Key)
		}
		t.Key = model.FormatKey(ws.KeyPrefix, used[ws.KeyPrefix])
	}
	return nil
}

// oldKey is the key and former keys a task had before assignKeys.
type oldKey struct {
	t      *model.Task
	key    string
	former []string
}

// restoreKeys puts back the keys assignKeys changed.
func (tx *Tx) restoreKeys() {
	for _, o := range tx.oldKeys {
		o.t.Key, o.t.FormerKeys = o.key, o.former
	}
}
