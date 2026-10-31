package store

import (
	"errors"
	"slices"

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
// over the one at HEAD.
func (tx *Tx) siblings(t *model.Task) ([]*model.Task, error) {
	var ulids []model.ULID
	if f := tx.s.siblings; f != nil {
		var err error
		if ulids, err = f(t.Parent, t.Workspace); err != nil {
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
	for _, ulid := range tx.taskOrder {
		if !slices.Contains(ulids, ulid) {
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
		if o.DeletedAt == nil && o.Workspace == t.Workspace && sameParent(o.Parent, t.Parent) {
			out = append(out, o)
		}
	}
	return out, nil
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
			keys[i] = model.RankKey{ID: o.ID, ULID: o.ULID, Rank: o.Rank}
		}
		r, err := model.RankAmong(model.RankKey{ID: t.ID, ULID: t.ULID, Rank: t.Rank}, keys)
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
