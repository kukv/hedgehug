package tasks

import (
	"errors"
	"slices"

	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// PatchInput is what PATCH /tasks/{id} changes: Assigns as hedgehug set
// takes them, and the body when Body is not nil.
type PatchInput struct {
	Assigns []usecase.Assign
	Body    *string
}

// Patch makes the changes of in to the task arg names in one commit. Its op
// is edit when only the body changes, set otherwise, as hedgehug edit and
// hedgehug set name theirs.
func Patch(ss *usecase.Session, arg string, in PatchInput) (usecase.TaskResult, error) {
	if len(in.Assigns) == 0 && in.Body == nil {
		return usecase.TaskResult{}, usecase.NewErr(usecase.KindUsage, "patch.empty", errors.New("nothing to change"))
	}
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
	var host string
	if in.Body != nil {
		if host, err = ss.WriterHost(); err != nil {
			return usecase.TaskResult{}, err
		}
	}
	op := "set"
	if len(in.Assigns) == 0 {
		op = "edit"
	}
	hash, err := ss.UpdateCommit(s, store.Op{Name: op, Task: e.ULID}, func(tx *store.Tx) error {
		t, err := tx.Task(e.ULID)
		if err != nil {
			return err
		}
		assignees := slices.Clone(t.Assignees)
		for _, as := range in.Assigns {
			if err := usecase.ApplyAssign(t, as, ix.Schema(), ss.ResolveULID(ix)); err != nil {
				return err
			}
		}
		usecase.WatchNewAssignees(t, assignees)
		if in.Body != nil {
			old := t.Body
			t.Body = *in.Body
			usecase.AddWatchers(t, usecase.NewMentions(old, t.Body, host)...)
		}
		return tx.PutTask(t)
	})
	if err != nil {
		return usecase.TaskResult{}, err
	}
	return usecase.NewTaskResult(ix, e.ULID, hash), nil
}
