package store

import (
	"errors"

	"github.com/kukv/hedgehug/internal/model"
)

// Siblings returns the tasks at HEAD under parent (nil for the top level)
// in workspace ws, deleted ones included. Update calls it under the lock.
type Siblings func(parent *model.ULID, ws model.ULID) ([]model.ULID, error)

// SetSiblings makes Update find siblings with f instead of reading every
// task file. The index sets it.
func (s *Store) SetSiblings(f Siblings) { s.siblings = f }

// siblings returns the live tasks that share t's parent and workspace as
// the operation leaves them, t excluded. A task put in the operation wins
// over the one at HEAD. A task whose parent does not exist is at the top
// level, as the index shows it.
func (tx *Tx) siblings(t *model.Task) ([]*model.Task, error) {
	parent, err := tx.effectiveParent(t)
	if err != nil {
		return nil, err
	}
	var ulids []model.ULID
	if f := tx.s.siblings; f != nil {
		if ulids, err = f(parent, t.Workspace); err != nil {
			return nil, err
		}
	} else {
		for o, err := range tx.s.Tasks() {
			var fe *FileError
			if errors.As(err, &fe) {
				continue // it cannot be read, so it has no rank to share
			}
			if err != nil {
				return nil, err
			}
			ulids = append(ulids, o.ULID)
		}
	}
	seen := make(map[model.ULID]bool, len(ulids))
	for _, ulid := range ulids {
		seen[ulid] = true
	}
	for _, ulid := range tx.taskOrder {
		if !seen[ulid] {
			ulids = append(ulids, ulid)
		}
	}
	var out []*model.Task
	for _, ulid := range ulids {
		if ulid == t.ULID {
			continue
		}
		o, err := tx.task(ulid)
		var de *model.DecodeError
		if errors.Is(err, ErrNotFound) || errors.As(err, &de) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if o.DeletedAt != nil || o.Workspace != t.Workspace {
			continue
		}
		op, err := tx.effectiveParent(o)
		if err != nil {
			return nil, err
		}
		if sameParent(op, parent) {
			out = append(out, o)
		}
	}
	return out, nil
}

// effectiveParent returns the parent of t, or nil when t has none or the
// parent does not exist.
func (tx *Tx) effectiveParent(t *model.Task) (*model.ULID, error) {
	if t.Parent == nil {
		return nil, nil
	}
	_, err := tx.task(*t.Parent)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	var de *model.DecodeError
	if err != nil && !errors.As(err, &de) {
		return nil, err
	}
	return t.Parent, nil
}

func sameParent(a, b *model.ULID) bool {
	return (a == nil) == (b == nil) && (a == nil || *a == *b)
}

// oldRank is the rank a task had before rerank.
type oldRank struct {
	t    *model.Task
	rank model.Rank
}

// rerank gives a task put in the operation a rank of its own when a sibling
// has the same rank: the design orders such siblings by id and fixes the
// rank at the next write. Only the written task changes; see model.RankAmong.
func (tx *Tx) rerank() error {
	for _, ulid := range tx.taskOrder {
		t := tx.tasks[ulid]
		if t == nil || t.DeletedAt != nil || t.Rank == "" {
			continue // validate reports a missing rank
		}
		sibs, err := tx.siblings(t)
		if err != nil {
			return err
		}
		keys := make([]model.RankKey, len(sibs))
		for i, o := range sibs {
			keys[i] = o.RankKey()
		}
		r, err := model.RankAmong(t.RankKey(), keys)
		if err != nil {
			return err
		}
		if r != t.Rank {
			tx.oldRanks = append(tx.oldRanks, oldRank{t, t.Rank})
			t.Rank = r
		}
	}
	return nil
}
