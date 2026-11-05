package tasks

import (
	"errors"
	"fmt"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// Rerank puts a task where order, the list a screen shows after a drop, has
// it. The list crosses parents, as the backlog does: no parent changes, and
// the order of the tree may change too. Ranks are given per parent, so the
// tasks after it may not lie past the rank before it: those get new ranks as
// well, in the same commit.
func Rerank(ss *usecase.Session, arg string, order []string) (usecase.TaskResult, error) {
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return usecase.TaskResult{}, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return usecase.TaskResult{}, err
	}
	if err := usecase.NotDeleted(ix, e); err != nil {
		return usecase.TaskResult{}, err
	}
	list, at, err := rankList(ss, ix, e, order)
	if err != nil {
		return usecase.TaskResult{}, err
	}
	targets, low, high := rankRun(list, at)
	if len(targets) == 1 && e.Rank > low && (high == "" || e.Rank < high) {
		return usecase.NewTaskResult(ix, e.ULID, plumbing.ZeroHash), nil
	}
	hash, err := ss.UpdateCommit(s, store.Op{Name: "mv", Task: e.ULID}, func(tx *store.Tx) error {
		r := low
		for _, x := range targets {
			var err error
			if r, err = model.Between(r, high); err != nil {
				return err
			}
			t, err := tx.Task(x.ULID)
			if err != nil {
				return err
			}
			t.Rank = r
			if err := tx.PutTask(t); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return usecase.TaskResult{}, err
	}
	return usecase.NewTaskResult(ix, e.ULID, hash), nil
}

// rankList reads order: live tasks of the workspace of e, each once, e among
// them. at is where e stands.
func rankList(ss *usecase.Session, ix *index.Index, e *index.Entry, order []string) ([]*index.Entry, int, error) {
	invalid := func(err error) error { return usecase.NewErr(usecase.KindInvalid, "rank.invalid", err) }
	list := make([]*index.Entry, 0, len(order))
	seen := map[model.ULID]bool{}
	at := -1
	for _, ref := range order {
		x, err := ss.ResolveRef(ix, ref)
		if err != nil {
			return nil, 0, err
		}
		if x.Workspace != e.Workspace || x.DeletedAt != nil {
			return nil, 0, invalid(fmt.Errorf("%s is not a live task of the workspace", usecase.RefName(ix, x.ULID)))
		}
		if seen[x.ULID] {
			return nil, 0, invalid(fmt.Errorf("%s is in the order twice", usecase.RefName(ix, x.ULID)))
		}
		seen[x.ULID] = true
		if x.ULID == e.ULID {
			at = len(list)
		}
		list = append(list, x)
	}
	if at < 0 {
		return nil, 0, invalid(errors.New("the task is not in the order"))
	}
	return list, at, nil
}

// rankRun says which tasks of list need a new rank to put the one at at in
// its place, and the ranks they lie between: it, and the tasks after it that
// are not past the rank before it.
func rankRun(list []*index.Entry, at int) (targets []*index.Entry, low, high model.Rank) {
	if at > 0 {
		low = list[at-1].Rank
	}
	targets = []*index.Entry{list[at]}
	i := at + 1
	for ; i < len(list) && list[i].Rank <= low; i++ {
		targets = append(targets, list[i])
	}
	if i < len(list) {
		high = list[i].Rank
	}
	return targets, low, high
}
