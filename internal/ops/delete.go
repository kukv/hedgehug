package ops

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// AffectedResult is what rm, restore and purge return: the task and the
// descendants that went with it. Index is the one read before the write; the
// CLI refreshes it to print what the write left.
type AffectedResult struct {
	Index     *index.Index
	Entry     *index.Entry
	Targets   []*index.Entry
	Committed bool
	Commit    string // hex hash of the commit made; "" when nothing was committed
}

// Views are the tasks of Targets as the write left them, refreshed from the
// store; a purged task, which is gone, is as it was.
func (r AffectedResult) Views() ([]*TaskView, error) {
	if err := r.Index.Refresh(); err != nil {
		return nil, err
	}
	out := make([]*TaskView, 0, len(r.Targets))
	for _, x := range r.Targets {
		cur, ok := r.Index.Task(x.ULID)
		if !ok {
			cur = x
		}
		out = append(out, TaskViewOf(r.Index, cur))
	}
	return out, nil
}

// Descendants returns the tasks under ulid for which follow is true, going
// down only through those, depth first in rank order. A parent cycle stops
// the walk.
func Descendants(ix *index.Index, ulid model.ULID, follow func(*index.Entry) bool) []*index.Entry {
	seen := map[model.ULID]bool{ulid: true}
	var out []*index.Entry
	var walk func(model.ULID)
	walk = func(p model.ULID) {
		for _, c := range ix.Children(p) {
			if seen[c.ULID] || !follow(c) {
				continue
			}
			seen[c.ULID] = true
			out = append(out, c)
			walk(c.ULID)
		}
	}
	walk(ulid)
	return out
}

func every(*index.Entry) bool { return true }

// Remove deletes the task arg names and the tasks under it; they can be
// restored.
func (ss *Session) Remove(arg string) (AffectedResult, error) {
	ts, arg, err := ss.forTask(arg)
	if err != nil {
		return AffectedResult{}, err
	}
	if ts != ss {
		return ts.Remove(arg)
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return AffectedResult{}, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return AffectedResult{}, err
	}
	if err := notDeletedYet(ix, e); err != nil {
		return AffectedResult{}, err
	}
	var targets []*index.Entry
	now := ss.Now().Truncate(time.Second)
	hash, err := ss.UpdateCommit(s, store.Op{Name: "rm", Task: e.ULID}, func(tx *store.Tx) error {
		// Read under the lock, so a child added since is deleted too, and
		// a task deleted since is not deleted again.
		var err error
		if e, err = current(ix, e); err != nil {
			return err
		}
		if err := notDeletedYet(ix, e); err != nil {
			return err
		}
		targets = []*index.Entry{e}
		for _, d := range Descendants(ix, e.ULID, every) {
			if d.DeletedAt == nil {
				targets = append(targets, d)
			}
		}
		for _, x := range targets {
			t, err := tx.Task(x.ULID)
			if err != nil {
				return err
			}
			t.DeletedAt = &now
			if err := tx.PutTask(t); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return AffectedResult{}, err
	}
	return AffectedResult{Index: ix, Entry: e, Targets: targets, Committed: !hash.IsZero(), Commit: commitHex(hash)}, nil
}

// Restore restores the deleted task arg names and what the same Remove
// deleted under it.
func (ss *Session) Restore(arg string) (AffectedResult, error) {
	ts, arg, err := ss.forTask(arg)
	if err != nil {
		return AffectedResult{}, err
	}
	if ts != ss {
		return ts.Restore(arg)
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return AffectedResult{}, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return AffectedResult{}, err
	}
	if err := restorable(ix, e); err != nil {
		return AffectedResult{}, err
	}
	var targets []*index.Entry
	hash, err := ss.UpdateCommit(s, store.Op{Name: "restore", Task: e.ULID}, func(tx *store.Tx) error {
		// Read under the lock, so a child the same rm deleted is restored
		// even when it came in after the index was read, and a task
		// restored since is refused.
		var err error
		if e, err = current(ix, e); err != nil {
			return err
		}
		if err := restorable(ix, e); err != nil {
			return err
		}
		at := *e.DeletedAt
		targets = append([]*index.Entry{e}, Descendants(ix, e.ULID, func(d *index.Entry) bool {
			return d.DeletedAt != nil && d.DeletedAt.Equal(at)
		})...)
		for _, x := range targets {
			t, err := tx.Task(x.ULID)
			if err != nil {
				return err
			}
			t.DeletedAt = nil
			if err := tx.PutTask(t); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return AffectedResult{}, err
	}
	return AffectedResult{Index: ix, Entry: e, Targets: targets, Committed: !hash.IsZero(), Commit: commitHex(hash)}, nil
}

// PurgeTargets lists what Purge would remove, for the caller to confirm: the
// index, the task arg names and the tasks under it. It refuses when one of
// them is not deleted.
func (ss *Session) PurgeTargets(arg string) (*index.Index, *index.Entry, []*index.Entry, error) {
	_, ix, err := ss.OpenForWrite()
	if err != nil {
		return nil, nil, nil, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return nil, nil, nil, err
	}
	targets, err := purgeTargets(ix, e)
	if err != nil {
		return nil, nil, nil, err
	}
	return ix, e, targets, nil
}

// Purge removes the files of the task arg names and of the tasks under it for
// good. confirmed is the ULIDs of the tasks the caller showed to the user
// (PurgeTargets lists them) and the user agreed to: it is required, and Purge
// fails with purge.changed when the tasks under it are no longer those.
func (ss *Session) Purge(arg string, confirmed []model.ULID) (AffectedResult, error) {
	if len(confirmed) == 0 {
		return AffectedResult{}, confirmRequired("purge " + arg)
	}
	return ss.purge(arg, confirmed)
}

// PurgeUnchecked is Purge for a purge that was not confirmed (hedgehug purge
// --yes): it works from the tasks as they are under the lock.
func (ss *Session) PurgeUnchecked(arg string) (AffectedResult, error) {
	return ss.purge(arg, nil)
}

// purge is Purge; a nil confirmed skips the comparison.
func (ss *Session) purge(arg string, confirmed []model.ULID) (AffectedResult, error) {
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return AffectedResult{}, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return AffectedResult{}, err
	}
	now, err := purgeTargets(ix, e)
	if err != nil {
		return AffectedResult{}, err
	}
	hash, err := ss.UpdateCommit(s, store.Op{Name: "purge", Task: e.ULID}, func(tx *store.Tx) error {
		// Read again under the lock: a task restored or added under it
		// since must not be purged unchecked, nor left behind.
		var err error
		if e, err = current(ix, e); err != nil {
			return err
		}
		if now, err = purgeTargets(ix, e); err != nil {
			return err
		}
		if confirmed != nil && !sameTasks(now, confirmed) {
			return newErr(KindConflict, "purge.changed", errors.New("the tasks under it changed after the confirmation; run purge again"))
		}
		for _, x := range now {
			if err := tx.RemoveTask(x.ULID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return AffectedResult{}, err
	}
	return AffectedResult{Index: ix, Entry: e, Targets: now, Committed: !hash.IsZero(), Commit: commitHex(hash)}, nil
}

// current refreshes ix, as an operation does under the lock, and returns e as
// it is now.
func current(ix *index.Index, e *index.Entry) (*index.Entry, error) {
	if err := ix.Refresh(); err != nil {
		return nil, err
	}
	cur, ok := ix.Task(e.ULID)
	if !ok {
		return nil, newErr(KindNotFound, "task.gone", fmt.Errorf("%s is no longer in the store", TaskLabel(ix, e)), "id", TaskFileName(e))
	}
	return cur, nil
}

// notDeletedYet refuses to delete the task e again.
func notDeletedYet(ix *index.Index, e *index.Entry) error {
	if e.DeletedAt != nil {
		return newErr(KindConflict, "task.already_deleted", fmt.Errorf("%s is already deleted", RefName(ix, e.ULID)), "id", TaskFileName(e))
	}
	return nil
}

// isDeleted refuses to restore or purge the task e when it is not deleted;
// hint ends the message.
func isDeleted(ix *index.Index, e *index.Entry, hint string) error {
	if e.DeletedAt == nil {
		return newErr(KindConflict, "task.not_deleted", fmt.Errorf("%s is not deleted%s", RefName(ix, e.ULID), hint), "id", TaskFileName(e))
	}
	return nil
}

// restorable refuses to restore e when it is not deleted or its parent is.
func restorable(ix *index.Index, e *index.Entry) error {
	if err := isDeleted(ix, e, ""); err != nil {
		return err
	}
	if e.Parent != nil {
		if p, ok := ix.Task(*e.Parent); ok {
			return LiveParent(ix, p)
		}
	}
	return nil
}

// purgeTargets returns e and every task under it, refusing when one of them
// is not deleted.
func purgeTargets(ix *index.Index, e *index.Entry) ([]*index.Entry, error) {
	if err := isDeleted(ix, e, "; delete it with hedgehug rm first"); err != nil {
		return nil, err
	}
	targets := append([]*index.Entry{e}, Descendants(ix, e.ULID, every)...)
	for _, d := range targets[1:] {
		if d.DeletedAt == nil {
			return nil, newErr(KindConflict, "purge.child_not_deleted", fmt.Errorf("%s under it is not deleted", RefName(ix, d.ULID)), "id", TaskFileName(d))
		}
	}
	return targets, nil
}

// sameTasks reports whether a and b hold the same tasks.
func sameTasks(a []*index.Entry, b []model.ULID) bool {
	ulids := func(n int, at func(int) model.ULID) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = at(i).String()
		}
		slices.Sort(out)
		return out
	}
	return slices.Equal(
		ulids(len(a), func(i int) model.ULID { return a[i].ULID }),
		ulids(len(b), func(i int) model.ULID { return b[i] }))
}

// ParseAtTime reads at as a time: RFC 3339, or a date, which stands for the
// end of that day in loc.
func ParseAtTime(at string, loc *time.Location) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339, at); err == nil {
		return t, true
	}
	if d, err := time.ParseInLocation(time.DateOnly, at, loc); err == nil {
		return d.Add(24*time.Hour - time.Second), true
	}
	return time.Time{}, false
}

// commitAt returns the commit whose state RestoreAt reads: for a time, the
// newest commit on the first-parent line from HEAD made at or before it (the
// zero hash when there is none); for anything else, the commit it names, an
// abbreviated hash included.
func commitAt(r *gitrepo.Repo, at string, loc *time.Location) (plumbing.Hash, error) {
	t, ok := ParseAtTime(at, loc)
	if !ok {
		h, err := r.ResolveCommit(at)
		if err != nil {
			return plumbing.ZeroHash, newErr(KindInvalid, "restore_at.bad_time", fmt.Errorf("%q is neither a time nor a commit", at), "at", at)
		}
		return h, nil
	}
	head, err := r.Head()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	log, err := r.LogFrom(head, 0)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	for _, e := range log {
		if !e.When.After(t) {
			return e.Hash, nil
		}
	}
	return plumbing.ZeroHash, nil
}

// workspaceName is the name of the workspace, or its ULID when the store has
// no such workspace now.
func workspaceName(ix *index.Index, ulid model.ULID) string {
	if w, ok := ix.Workspace(ulid); ok {
		return w.Name
	}
	return ulid.String()
}

// RestoreAtResult is what RestoreAt did. Note is a line for the reader when
// the task did not come back whole: its workspace stays.
type RestoreAtResult struct {
	TaskResult
	Note string
}

// RestoreAt puts the task back as it was at the point at names (a time or a
// commit). Its id and workspace stay as they are now: an id given back could
// be given twice, and moving to another workspace is the job of Move with a
// workspace, which moves the tasks under it too.
func (ss *Session) RestoreAt(arg, at string) (RestoreAtResult, error) {
	ts, arg, err := ss.forTask(arg)
	if err != nil {
		return RestoreAtResult{}, err
	}
	if ts != ss {
		return ts.RestoreAt(arg, at)
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return RestoreAtResult{}, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return RestoreAtResult{}, err
	}
	commit, err := commitAt(s.Repo(), at, ss.location())
	if err != nil {
		return RestoreAtResult{}, err
	}
	var note string
	hash, err := ss.UpdateCommit(s, store.Op{Name: "restore", Task: e.ULID}, func(tx *store.Tx) error {
		// Read under the lock, as the other writes do.
		var err error
		if e, err = current(ix, e); err != nil {
			return err
		}
		data, err := s.Repo().ReadAt(commit, model.TaskPath(e.ULID))
		if errors.Is(err, fs.ErrNotExist) {
			return newErr(KindNotFound, "restore_at.not_found", fmt.Errorf("%s did not exist at %s", cmp.Or(ix.DisplayID(e), e.ULID.String()), at), "at", at)
		}
		if err != nil {
			return err
		}
		old, err := model.DecodeTask(data)
		if err != nil {
			return err
		}
		if old.DeletedAt == nil && old.Parent != nil {
			if p, ok := ix.Task(*old.Parent); ok {
				if err := LiveParent(ix, p); err != nil {
					return err
				}
			}
		}
		cur, err := tx.Task(e.ULID)
		if err != nil {
			return err
		}
		old.ID, old.FormerIDs = cur.ID, cur.FormerIDs
		if old.Workspace != cur.Workspace {
			note = fmt.Sprintf("the workspace stays %s; move it with hedgehug mv %s --workspace %s",
				workspaceName(ix, cur.Workspace), ix.QualifyID(cur.ID), workspaceName(ix, old.Workspace))
			old.Workspace = cur.Workspace
		}
		return tx.PutTask(old)
	})
	if err != nil {
		return RestoreAtResult{}, err
	}
	return RestoreAtResult{TaskResult: TaskResult{Index: ix, ULID: e.ULID, Committed: !hash.IsZero(), Commit: commitHex(hash)}, Note: note}, nil
}
