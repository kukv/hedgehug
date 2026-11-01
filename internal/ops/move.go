package ops

import (
	"errors"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// MoveInput is what Move moves a task by. Arg, Parent, Before and After are
// task names and Workspace a workspace name, as the user gives them.
type MoveInput struct {
	Arg, Workspace, Parent string
	Root                   bool
	Before, After          string
}

// MoveResult is what Move did: the tasks, in order, with the ids they had
// (OldIDs; the tasks hold their new ids), and whether they went to another
// workspace.
type MoveResult struct {
	TaskResult
	Tasks  []*model.Task
	OldIDs []string
	Moved  bool
	// Detached is what the move took off the tasks that went to another
	// workspace: the sprint and the releases that are not of the new one.
	// Only the tasks that lost something are listed.
	Detached []Detached
}

// Detached is the sprint and the releases a move took off one task, by name;
// a sprint or release whose file cannot be read is named by its ULID.
type Detached struct {
	Task     model.ULID `json:"task"`
	ID       string     `json:"id" doc:"The id of the task after the move"`
	Sprint   string     `json:"sprint,omitempty" doc:"The name of the sprint taken off"`
	Releases []string   `json:"releases,omitempty" doc:"The names of the releases taken off"`
}

// Move moves a task under another parent, to another workspace, or among its
// siblings.
func (ss *Session) Move(in MoveInput) (MoveResult, error) {
	ts, arg, err := ss.forTask(in.Arg)
	if err != nil {
		return MoveResult{}, err
	}
	if ts != ss {
		in.Arg = arg
		return ts.Move(in)
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return MoveResult{}, err
	}
	e, err := ss.Resolve(ix, in.Arg)
	if err != nil {
		return MoveResult{}, err
	}
	// The checks come in the order of the arguments; moveTargetOf repeats
	// them.
	if err := NotDeleted(ix, e); err != nil {
		return MoveResult{}, err
	}
	var p *index.Entry
	if in.Parent != "" {
		if p, err = ss.ResolveRef(ix, in.Parent); err != nil {
			return MoveResult{}, err
		}
		if err := LiveParent(ix, p); err != nil {
			return MoveResult{}, err
		}
	}
	var w *model.Workspace
	if in.Workspace != "" {
		if w, err = ResolveWorkspace(ix, in.Workspace); err != nil {
			return MoveResult{}, err
		}
	}
	if _, err := ss.moveTargetOf(ix, e, p, w, in.Root, in.Before, in.After); err != nil {
		return MoveResult{}, err
	}
	var res MoveResult
	hash, err := ss.UpdateCommit(s, store.Op{Name: "mv", Task: e.ULID}, func(tx *store.Tx) error {
		var err error
		res, err = ss.MoveInTx(tx, ix, e, p, w, in.Root, in.Before, in.After)
		return err
	})
	if err != nil {
		return MoveResult{}, err
	}
	for i := range res.Detached {
		for _, t := range res.Tasks {
			if t.ULID == res.Detached[i].Task {
				res.Detached[i].ID = t.ID
			}
		}
	}
	res.TaskResult = TaskResult{Index: ix, ULID: e.ULID, Committed: !hash.IsZero(), Commit: commitHex(hash)}
	return res, nil
}

// MoveInTx moves e as Move does, in the operation tx: under the new parent p
// (or to the top level for root), into the workspace w, before or after a
// sibling.
func (ss *Session) MoveInTx(tx *store.Tx, ix *index.Index, e, p *index.Entry, w *model.Workspace, root bool, before, after string) (MoveResult, error) {
	// Work out the move again under the lock, from the tasks as they
	// are now: another process may have moved or deleted them since
	// the index was read.
	var res MoveResult
	var err error
	if e, err = current(ix, e); err != nil {
		return res, err
	}
	if p != nil {
		if p, err = current(ix, p); err != nil {
			return res, err
		}
	}
	to, err := ss.moveTargetOf(ix, e, p, w, root, before, after)
	if err != nil {
		return res, err
	}
	// A task and its subtree are in one workspace, so they move
	// together, a child added since the index was read too.
	ulids := []model.ULID{e.ULID}
	if res.Moved = to.workspace != e.Workspace; res.Moved {
		for _, d := range Descendants(ix, e.ULID, func(*index.Entry) bool { return true }) {
			ulids = append(ulids, d.ULID)
		}
	}
	for _, ulid := range ulids {
		t, err := tx.Task(ulid)
		if err != nil {
			return res, err
		}
		res.OldIDs = append(res.OldIDs, t.ID)
		t.Workspace = to.workspace
		if ulid == e.ULID {
			t.Parent, t.Rank = to.parent, to.rank
		}
		if res.Moved {
			d, err := detachFromOtherWorkspaces(tx, t)
			if err != nil {
				return res, err
			}
			if d.Sprint != "" || len(d.Releases) > 0 {
				res.Detached = append(res.Detached, d)
			}
		}
		if err := tx.PutTask(t); err != nil {
			return res, err
		}
		res.Tasks = append(res.Tasks, t)
	}
	return res, nil
}

// detachFromOtherWorkspaces takes off t, which is now in its new workspace,
// the sprint and the releases that are not of that workspace, and says what it
// took. A sprint or release that is gone or cannot be read is not known to be
// of the workspace, so it goes too.
func detachFromOtherWorkspaces(tx *store.Tx, t *model.Task) (Detached, error) {
	d := Detached{Task: t.ULID}
	if t.Sprint != nil {
		sp, err := tx.Sprint(*t.Sprint)
		switch {
		case err == nil && sp.Workspace == t.Workspace:
		case err == nil:
			d.Sprint, t.Sprint = sp.Name, nil
		case unreadable(err):
			d.Sprint, t.Sprint = t.Sprint.String(), nil
		default:
			return d, err
		}
	}
	var kept []model.ULID
	for _, u := range t.Releases {
		r, err := tx.Release(u)
		switch {
		case err == nil && r.Workspace == t.Workspace:
			kept = append(kept, u)
		case err == nil:
			d.Releases = append(d.Releases, r.Name)
		case unreadable(err):
			d.Releases = append(d.Releases, u.String())
		default:
			return d, err
		}
	}
	if len(d.Releases) > 0 {
		t.Releases = kept
	}
	return d, nil
}

// unreadable reports whether err says a sprint or release file is missing or
// does not decode.
func unreadable(err error) bool {
	return errors.Is(err, store.ErrNotFound) || errors.As(err, new(*model.DecodeError))
}

// moveTarget is where Move puts a task.
type moveTarget struct {
	parent    *model.ULID
	workspace model.ULID
	rank      model.Rank
}

// moveTargetOf works out where Move puts e, as ix has the tasks: at the top
// level when root, or under the new parent p (nil to keep the parent), in
// the workspace w (nil to follow the parent), before or after a sibling.
func (ss *Session) moveTargetOf(ix *index.Index, e, p *index.Entry, w *model.Workspace, root bool, before, after string) (moveTarget, error) {
	if err := NotDeleted(ix, e); err != nil {
		return moveTarget{}, err
	}
	to := moveTarget{parent: e.Parent, workspace: e.Workspace}
	switch {
	case root:
		to.parent = nil
	case p != nil:
		if err := LiveParent(ix, p); err != nil {
			return moveTarget{}, err
		}
		to.parent, to.workspace = &p.ULID, p.Workspace
	}
	if w != nil {
		switch {
		case p != nil && w.ULID != to.workspace:
			pw, ok := ix.Workspace(to.workspace)
			if !ok {
				return moveTarget{}, newErr(KindInvalid, "move.invalid", fmt.Errorf("the parent's workspace %s is not in this store", to.workspace))
			}
			return moveTarget{}, newErr(KindInvalid, "move.invalid", fmt.Errorf("the parent is in workspace %s", pw.Name))
		case p == nil && w.ULID != e.Workspace:
			// The old parent stays behind, so the task goes to the top level.
			to.parent = nil
		}
		to.workspace = w.ULID
	}
	var err error
	to.rank, err = ss.rankAt(ix, e.ULID, to.parent, to.workspace, before, after)
	return to, err
}

// rankAt returns the rank for ulid under parent in workspace ws: just before
// or after a sibling, or after every sibling. Siblings that share a rank are
// ordered by id, so no rank lies between them: next to one of a group, the
// task goes past the group on the side asked for.
func (ss *Session) rankAt(ix *index.Index, ulid model.ULID, parent *model.ULID, ws model.ULID, before, after string) (model.Rank, error) {
	if before == "" && after == "" {
		return EndRank(ix, parent, ws, ulid)
	}
	ref := before
	if ref == "" {
		ref = after
	}
	x, err := ss.ResolveRef(ix, ref)
	if err != nil {
		return "", err
	}
	sib := Siblings(ix, parent, ws, ulid)
	if !slices.ContainsFunc(sib, func(e *index.Entry) bool { return e.ULID == x.ULID }) {
		return "", newErr(KindInvalid, "move.invalid", fmt.Errorf("%s is not under the same parent", RefName(ix, x.ULID)))
	}
	var lower, higher model.Rank
	for _, e := range sib {
		switch {
		case e.Rank < x.Rank && e.Rank > lower:
			lower = e.Rank
		case e.Rank > x.Rank && (higher == "" || e.Rank < higher):
			higher = e.Rank
		}
	}
	if before != "" {
		return model.Between(lower, x.Rank)
	}
	return model.Between(x.Rank, higher)
}
