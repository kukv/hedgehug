package tasks

import (
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// MoveInput is what Move moves a task by. Arg, Parent, Before and After are
// task names and Workspace a workspace name, as the user gives them.
type MoveInput struct {
	Arg, Workspace, Parent string
	Root                   bool
	Before, After          string
}

// Move moves a task under another parent, to another workspace, or among its
// siblings.
func Move(ss *usecase.Session, in MoveInput) (usecase.MoveResult, error) {
	ts, arg, err := ss.ForTask(in.Arg)
	if err != nil {
		return usecase.MoveResult{}, err
	}
	if ts != ss {
		in.Arg = arg
		return Move(ts, in)
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return usecase.MoveResult{}, err
	}
	e, err := ss.Resolve(ix, in.Arg)
	if err != nil {
		return usecase.MoveResult{}, err
	}
	// The checks come in the order of the arguments; moveTargetOf repeats
	// them.
	if err := usecase.NotDeleted(ix, e); err != nil {
		return usecase.MoveResult{}, err
	}
	var p *index.Entry
	if in.Parent != "" {
		if p, err = ss.ResolveRef(ix, in.Parent); err != nil {
			return usecase.MoveResult{}, err
		}
		if err := usecase.LiveParent(ix, p); err != nil {
			return usecase.MoveResult{}, err
		}
	}
	var w *model.Workspace
	if in.Workspace != "" {
		if w, err = usecase.ResolveWorkspace(ix, in.Workspace); err != nil {
			return usecase.MoveResult{}, err
		}
	}
	if _, err := ss.MoveTargetOf(ix, e, p, w, in.Root, in.Before, in.After); err != nil {
		return usecase.MoveResult{}, err
	}
	var res usecase.MoveResult
	hash, err := ss.UpdateCommit(s, store.Op{Name: "mv", Task: e.ULID}, func(tx *store.Tx) error {
		var err error
		res, err = ss.MoveInTx(tx, ix, e, p, w, in.Root, in.Before, in.After)
		return err
	})
	if err != nil {
		return usecase.MoveResult{}, err
	}
	for i := range res.Detached {
		for _, t := range res.Tasks {
			if t.ULID == res.Detached[i].Task {
				res.Detached[i].ID = t.ID
			}
		}
	}
	res.TaskResult = usecase.TaskResult{Index: ix, ULID: e.ULID, Committed: !hash.IsZero(), Commit: usecase.CommitHex(hash)}
	return res, nil
}
