package usecase

import (
	"errors"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
)

// MoveInTx moves e as Move does, in the operation tx: under the new parent p
// (or to the top level for root), into the workspace w, before or after a
// sibling.
func (ss *Session) MoveInTx(tx *store.Tx, ix *index.Index, e, p *index.Entry, w *model.Workspace, root bool, before, after string) (MoveResult, error) {
	// Work out the move again under the lock, from the tasks as they
	// are now: another process may have moved or deleted them since
	// the index was read.
	var res MoveResult
	var err error
	if e, err = Refreshed(ix, e); err != nil {
		return res, err
	}
	if p != nil {
		if p, err = Refreshed(ix, p); err != nil {
			return res, err
		}
	}
	to, err := ss.MoveTargetOf(ix, e, p, w, root, before, after)
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

// MoveTargetOf works out where Move puts e, as ix has the tasks: at the top
// level when root, or under the new parent p (nil to keep the parent), in
// the workspace w (nil to follow the parent), before or after a sibling.
func (ss *Session) MoveTargetOf(ix *index.Index, e, p *index.Entry, w *model.Workspace, root bool, before, after string) (MoveTarget, error) {
	if err := NotDeleted(ix, e); err != nil {
		return MoveTarget{}, err
	}
	to := MoveTarget{parent: e.Parent, workspace: e.Workspace}
	switch {
	case root:
		to.parent = nil
	case p != nil:
		if err := LiveParent(ix, p); err != nil {
			return MoveTarget{}, err
		}
		to.parent, to.workspace = &p.ULID, p.Workspace
	}
	if w != nil {
		switch {
		case p != nil && w.ULID != to.workspace:
			pw, ok := ix.Workspace(to.workspace)
			if !ok {
				return MoveTarget{}, NewErr(KindInvalid, "move.invalid", fmt.Errorf("the parent's workspace %s is not in this store", to.workspace))
			}
			return MoveTarget{}, NewErr(KindInvalid, "move.invalid", fmt.Errorf("the parent is in workspace %s", pw.Name))
		case p == nil && w.ULID != e.Workspace:
			// The old parent stays behind, so the task goes to the top level.
			to.parent = nil
		}
		to.workspace = w.ULID
	}
	var err error
	to.rank, err = ss.RankAt(ix, e.ULID, to.parent, to.workspace, before, after, "move.invalid")
	if err == nil && ss.alreadyThere(ix, e, to, before, after) {
		// Nothing moves, so the rank stays and the store has nothing to commit.
		to.rank = e.Rank
	}
	return to, err
}

// Refreshed refreshes ix, as an operation does under the lock, and returns e as
// it is now.
func Refreshed(ix *index.Index, e *index.Entry) (*index.Entry, error) {
	if err := ix.Refresh(); err != nil {
		return nil, err
	}
	cur, ok := ix.Task(e.ULID)
	if !ok {
		return nil, NewErr(KindNotFound, "task.gone", fmt.Errorf("%s is no longer in the store", TaskLabel(ix, e)), "id", TaskFileName(e))
	}
	return cur, nil
}

// Detached is the sprint and the releases a move took off one task, by name;
// a sprint or release whose file cannot be read is named by its ULID.
type Detached struct {
	Task     model.ULID `json:"task"`
	ID       string     `json:"id" doc:"The id of the task after the move"`
	Sprint   string     `json:"sprint,omitempty" doc:"The name of the sprint taken off"`
	Releases []string   `json:"releases,omitempty" doc:"The names of the releases taken off"`
}

// unreadable reports whether err says a sprint or release file is missing or
// does not decode.
func unreadable(err error) bool {
	return errors.Is(err, store.ErrNotFound) || errors.As(err, new(*model.DecodeError))
}

// MoveTarget is where Move puts a task.
type MoveTarget struct {
	parent    *model.ULID
	workspace model.ULID
	rank      model.Rank
}

// alreadyThere says whether e, which RankAt has found a place for, already
// stands there: under the same parent in the same workspace, last when no
// sibling is named, or right after (before) the sibling named with a rank of
// its own. Tasks of one rank are ordered by id and the new rank lies past the
// whole group, so e next to a task of the same rank may still move.
func (ss *Session) alreadyThere(ix *index.Index, e *index.Entry, to MoveTarget, before, after string) bool {
	if to.workspace != e.Workspace || !sameParent(to.parent, e.Parent) {
		return false
	}
	sib := Siblings(ix, to.parent, to.workspace, model.ULID{})
	at := slices.IndexFunc(sib, func(x *index.Entry) bool { return x.ULID == e.ULID })
	if at < 0 {
		return false
	}
	ref := before
	if ref == "" {
		ref = after
	}
	if ref == "" {
		return at == len(sib)-1
	}
	x, err := ss.ResolveRef(ix, ref)
	if err != nil {
		return false
	}
	if before != "" {
		return at+1 < len(sib) && sib[at+1].ULID == x.ULID && e.Rank < x.Rank
	}
	return at > 0 && sib[at-1].ULID == x.ULID && e.Rank > x.Rank
}

// RankAt returns the rank for ulid under parent in workspace ws: just before
// or after a sibling, or after every sibling. Siblings that share a rank are
// ordered by id, so no rank lies between them: next to one of a group, the
// task goes past the group on the side asked for. Giving both before and
// after is position.both; a sibling that is not one (deleted, or under another
// parent or workspace) is the error code the caller gives, move.invalid for
// Move and add.position for Add.
func (ss *Session) RankAt(ix *index.Index, ulid model.ULID, parent *model.ULID, ws model.ULID, before, after, code string) (model.Rank, error) {
	if err := CheckPosition(before, after); err != nil {
		return "", err
	}
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
		if code == "add.position" {
			return "", NewErr(KindInvalid, code, fmt.Errorf("%s is not a sibling of the new task (it is deleted, or under another parent or workspace)", RefName(ix, x.ULID)))
		}
		return "", NewErr(KindInvalid, code, fmt.Errorf("%s is not under the same parent", RefName(ix, x.ULID)))
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

func sameParent(a, b *model.ULID) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

// CheckPosition refuses both before and after (position.both). The CLI calls
// it as soon as it has read the flags, before it asks or looks anything up.
func CheckPosition(before, after string) error {
	if before != "" && after != "" {
		return NewErr(KindUsage, "position.both", errors.New("give --before or --after, not both"))
	}
	return nil
}

// Siblings returns the tasks under parent (the root when nil, which also
// holds the tasks whose parent is missing, as Roots does) in workspace ws
// that are not deleted, leaving out skip, by rank.
func Siblings(ix *index.Index, parent *model.ULID, ws, skip model.ULID) []*index.Entry {
	var all []*index.Entry
	if parent == nil {
		all = ix.Roots()
	} else {
		all = ix.Children(*parent)
	}
	out := all[:0:0]
	for _, e := range all {
		if e.DeletedAt == nil && e.ULID != skip && e.Workspace == ws {
			out = append(out, e)
		}
	}
	return out
}

// EndRank returns a rank after every sibling under parent.
func EndRank(ix *index.Index, parent *model.ULID, ws, skip model.ULID) (model.Rank, error) {
	sib := Siblings(ix, parent, ws, skip)
	var last model.Rank
	if len(sib) > 0 {
		last = sib[len(sib)-1].Rank
	}
	return model.Between(last, "")
}
