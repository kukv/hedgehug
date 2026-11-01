package ops

import (
	"errors"
	"slices"

	"github.com/kukv/hedgehug/internal/store"
)

// PatchInput is what PATCH /tasks/{id} changes: Assigns as hedgehug set
// takes them, and the body when Body is not nil.
type PatchInput struct {
	Assigns []Assign
	Body    *string
}

// Patch makes the changes of in to the task arg names in one commit. Its op
// is edit when only the body changes, set otherwise, as hedgehug edit and
// hedgehug set name theirs.
func (ss *Session) Patch(arg string, in PatchInput) (TaskResult, error) {
	if len(in.Assigns) == 0 && in.Body == nil {
		return TaskResult{}, newErr(KindUsage, "patch.empty", errors.New("nothing to change"))
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return TaskResult{}, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return TaskResult{}, err
	}
	if err := NotDeleted(ix, e); err != nil {
		return TaskResult{}, err
	}
	var host string
	if in.Body != nil {
		if host, err = ss.WriterHost(); err != nil {
			return TaskResult{}, err
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
			if err := ApplyAssign(t, as, ix.Schema(), ss.ResolveULID(ix)); err != nil {
				return err
			}
		}
		watchNewAssignees(t, assignees)
		if in.Body != nil {
			old := t.Body
			t.Body = *in.Body
			AddWatchers(t, NewMentions(old, t.Body, host)...)
		}
		return tx.PutTask(t)
	})
	if err != nil {
		return TaskResult{}, err
	}
	return TaskResult{Index: ix, ULID: e.ULID, Committed: !hash.IsZero(), Commit: commitHex(hash)}, nil
}
