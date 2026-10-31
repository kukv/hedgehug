package main

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// parseAtTime reads at as a time: RFC 3339, or a date, which stands for the
// end of that day in loc.
func parseAtTime(at string, loc *time.Location) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339, at); err == nil {
		return t, true
	}
	if d, err := time.ParseInLocation(time.DateOnly, at, loc); err == nil {
		return d.Add(24*time.Hour - time.Second), true
	}
	return time.Time{}, false
}

// commitAt returns the commit whose state restore --at reads: for a time,
// the newest commit on the first-parent line from HEAD made at or before it
// (the zero hash when there is none); for anything else, the commit it
// names, an abbreviated hash included.
func commitAt(r *gitrepo.Repo, at string) (plumbing.Hash, error) {
	t, ok := parseAtTime(at, time.Local)
	if !ok {
		h, err := r.ResolveCommit(at)
		if err != nil {
			return plumbing.ZeroHash, fmt.Errorf("%q is neither a time nor a commit", at)
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

// restoreAt puts the task back as it was at the point at names. Its id and
// workspace stay as they are now: an id given back could be given twice, and
// moving to another workspace is the job of mv --workspace, which moves the
// tasks under it too.
func (a *app) restoreAt(arg, at string) error {
	s, ix, err := a.open()
	if err != nil {
		return err
	}
	e, err := a.resolve(ix, arg)
	if err != nil {
		return err
	}
	commit, err := commitAt(s.Repo(), at)
	if err != nil {
		return err
	}
	var note string
	committed, err := a.update(s, store.Op{Name: "restore", Task: e.ULID}, func(tx *store.Tx) error {
		// Read under the lock, as the other writes do.
		var err error
		if e, err = current(ix, e); err != nil {
			return err
		}
		data, err := s.Repo().ReadAt(commit, model.TaskPath(e.ULID))
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%s did not exist at %s", cmp.Or(e.ID, e.ULID.String()), at)
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
				if err := liveParent(ix, p); err != nil {
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
				workspaceName(ix, cur.Workspace), cur.ID, workspaceName(ix, old.Workspace))
			old.Workspace = cur.Workspace
		}
		return tx.PutTask(old)
	})
	if err != nil {
		return err
	}
	if err := a.printTask(ix, e.ULID, verb(committed, "Restored")); err != nil || note == "" {
		return err
	}
	// JSON on stdout stays one document.
	out := a.stdout
	if a.jsonOut {
		out = a.stderr
	}
	_, err = fmt.Fprintln(out, "  "+note)
	return err
}
