package tasks

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/upgrade"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// AffectedResult is what rm and restore return: the task and the
// descendants that went with it. Index is the one read before the write; the
// CLI refreshes it to print what the write left.
type AffectedResult struct {
	Index     *index.Index
	Entry     *index.Entry
	Targets   []*index.Entry
	Committed bool
	Commit    string // hex hash of the commit made; "" when nothing was committed
}

// Views are the tasks of Targets as the write left them, as rows of a list:
// the task first, then the tasks under it, each once. It refreshes the index.
func (r AffectedResult) Views() ([]*usecase.TaskView, error) {
	if err := r.Index.Refresh(); err != nil {
		return nil, err
	}
	ulids := []model.ULID{r.Entry.ULID}
	for _, x := range r.Targets {
		if x.ULID != r.Entry.ULID {
			ulids = append(ulids, x.ULID)
		}
	}
	out := make([]*usecase.TaskView, 0, len(ulids))
	for _, u := range ulids {
		cur, ok := r.Index.Task(u)
		if !ok {
			// Another process purged it after the write.
			return nil, usecase.NewErr(usecase.KindNotFound, "task.gone", fmt.Errorf("%s is no longer in the store", u), "id", u.String())
		}
		out = append(out, usecase.ListViewOf(r.Index, cur))
	}
	return out, nil
}

// Write is the answer to the write: the tasks as they are now, and the
// commit.
func (r AffectedResult) Write() (usecase.AffectedWrite, error) {
	vs, err := r.Views()
	if err != nil {
		return usecase.AffectedWrite{}, err
	}
	return usecase.AffectedWrite{Tasks: vs, Commit: usecase.CommitPtr(r.Commit)}, nil
}

// Remove deletes the task arg names and the tasks under it; they can be
// restored.
func Remove(ss *usecase.Session, arg string) (AffectedResult, error) {
	ts, arg, err := ss.ForTask(arg)
	if err != nil {
		return AffectedResult{}, err
	}
	if ts != ss {
		return Remove(ts, arg)
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
		if e, err = usecase.Refreshed(ix, e); err != nil {
			return err
		}
		if err := notDeletedYet(ix, e); err != nil {
			return err
		}
		targets = []*index.Entry{e}
		for _, d := range usecase.Descendants(ix, e.ULID, usecase.Every) {
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
	return AffectedResult{Index: ix, Entry: e, Targets: targets, Committed: !hash.IsZero(), Commit: usecase.CommitHex(hash)}, nil
}

// Restore restores the deleted task arg names and what the same Remove
// deleted under it.
func Restore(ss *usecase.Session, arg string) (AffectedResult, error) {
	ts, arg, err := ss.ForTask(arg)
	if err != nil {
		return AffectedResult{}, err
	}
	if ts != ss {
		return Restore(ts, arg)
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
		if e, err = usecase.Refreshed(ix, e); err != nil {
			return err
		}
		if err := restorable(ix, e); err != nil {
			return err
		}
		at := *e.DeletedAt
		targets = append([]*index.Entry{e}, usecase.Descendants(ix, e.ULID, func(d *index.Entry) bool {
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
	return AffectedResult{Index: ix, Entry: e, Targets: targets, Committed: !hash.IsZero(), Commit: usecase.CommitHex(hash)}, nil
}

// notDeletedYet refuses to delete the task e again.
func notDeletedYet(ix *index.Index, e *index.Entry) error {
	if e.DeletedAt != nil {
		return usecase.NewErr(usecase.KindConflict, "task.already_deleted", fmt.Errorf("%s is already deleted", usecase.RefName(ix, e.ULID)), "id", usecase.TaskFileName(e))
	}
	return nil
}

// isDeleted refuses to restore the task e when it is not deleted.
func isDeleted(ix *index.Index, e *index.Entry) error {
	if e.DeletedAt == nil {
		return usecase.NewErr(usecase.KindConflict, "task.not_deleted", fmt.Errorf("%s is not deleted", usecase.RefName(ix, e.ULID)), "id", usecase.TaskFileName(e))
	}
	return nil
}

// restorable refuses to restore e when it is not deleted or its parent is.
func restorable(ix *index.Index, e *index.Entry) error {
	if err := isDeleted(ix, e); err != nil {
		return err
	}
	if e.Parent != nil {
		if p, ok := ix.Task(*e.Parent); ok {
			return usecase.LiveParent(ix, p)
		}
	}
	return nil
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

// readTaskAt is the file of the task ulid as commit has it, in the newest
// store format: a commit from before a conversion is read through the steps
// of domain/upgrade. The error is fs.ErrNotExist when the commit lacks the task
// (and only then: a commit whose format cannot be read is another error).
func readTaskAt(s *store.Store, commit plumbing.Hash, ulid model.ULID) ([]byte, error) {
	r := s.Repo()
	data, err := r.ReadAt(commit, model.TaskPath(ulid))
	if err != nil {
		return nil, err
	}
	data, err = upgrade.At(func(p string) ([]byte, error) { return r.ReadAt(commit, p) }, r.ReadHead).Task(data)
	if errors.Is(err, fs.ErrNotExist) {
		// A commit without store.yaml is unreadable, which is not a task that did not exist.
		return nil, fmt.Errorf("the store format of %s cannot be read: %v", commit, err)
	}
	return data, err
}

// commitAt returns the commit whose state RestoreAt reads: for a time, the
// newest commit on the first-parent line from HEAD made at or before it (the
// zero hash when there is none); for anything else, the commit it names, an
// abbreviated hash included.
func commitAt(ss *usecase.Session, s *store.Store, name, at string) (plumbing.Hash, error) {
	r, loc := s.Repo(), ss.Location()
	t, ok := ParseAtTime(at, loc)
	if !ok {
		h, err := r.ResolveCommit(at)
		if err != nil {
			return plumbing.ZeroHash, usecase.NewErr(usecase.KindInvalid, "restore_at.bad_time", fmt.Errorf("%q is neither a time nor a commit", at), "at", at)
		}
		return h, nil
	}
	head, err := r.Head()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	// The history index is a cache: when it cannot be had, walk.
	if h, err := ss.OpenHistory(s, name); err == nil {
		if found, _, err := h.AtOrBefore(head, t); err == nil {
			return found, nil
		}
	}
	// Stop at the first commit made at or before t.
	found := plumbing.ZeroHash
	err = r.WalkFirstParents(head, func(e gitrepo.LogEntry) bool {
		if !e.When.After(t) {
			found = e.Hash
			return false
		}
		return true
	})
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return found, nil
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
	usecase.TaskResult
	Note string
}

// Write is the answer to the restore: the task as it is now, the commit and
// the note.
func (r RestoreAtResult) Write() (usecase.TaskWrite, error) {
	w, err := r.TaskResult.Write()
	if err != nil {
		return usecase.TaskWrite{}, err
	}
	w.Note = r.Note
	return w, nil
}

// RestoreAt puts the task back as it was at the point at names (a time or a
// commit). Its id and workspace stay as they are now: an id given back could
// be given twice, and moving to another workspace is the job of Move with a
// workspace, which moves the tasks under it too.
func RestoreAt(ss *usecase.Session, arg, at string) (RestoreAtResult, error) {
	ts, arg, err := ss.ForTask(arg)
	if err != nil {
		return RestoreAtResult{}, err
	}
	if ts != ss {
		return RestoreAt(ts, arg, at)
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return RestoreAtResult{}, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return RestoreAtResult{}, err
	}
	commit, err := commitAt(ss, s, ix.StoreName(), at)
	if err != nil {
		return RestoreAtResult{}, err
	}
	var note string
	hash, err := ss.UpdateCommit(s, store.Op{Name: "restore", Task: e.ULID}, func(tx *store.Tx) error {
		// Read under the lock, as the other writes do.
		var err error
		if e, err = usecase.Refreshed(ix, e); err != nil {
			return err
		}
		data, err := readTaskAt(s, commit, e.ULID)
		if errors.Is(err, fs.ErrNotExist) {
			return usecase.NewErr(usecase.KindNotFound, "restore_at.not_found", fmt.Errorf("%s did not exist at %s", cmp.Or(ix.DisplayID(e), e.ULID.String()), at), "at", at)
		}
		if err != nil {
			return err
		}
		old, err := model.DecodeTask(data)
		if errors.Is(err, model.ErrForward) {
			return usecase.NewErr(usecase.KindConflict, "restore.forward", fmt.Errorf("%s was moved to another store at that time", cmp.Or(ix.DisplayID(e), e.ULID.String())), "at", at)
		}
		if err != nil {
			return err
		}
		if old.DeletedAt == nil && old.Parent != nil {
			if p, ok := ix.Task(*old.Parent); ok {
				if err := usecase.LiveParent(ix, p); err != nil {
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
	return RestoreAtResult{TaskResult: usecase.NewTaskResult(ix, e.ULID, hash), Note: note}, nil
}
