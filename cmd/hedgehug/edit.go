package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/store"
)

func (a *app) editCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit the file of a task in $EDITOR",
		Args:  exactArgs(1),
		RunE:  func(_ *cobra.Command, args []string) error { return a.edit(args[0]) },
	}
}

var errChangedWhileEditing = errors.New("the task changed while editing")

func (a *app) edit(arg string) error {
	s, ix, err := a.ss.Open()
	if err != nil {
		return err
	}
	e, err := a.ss.Resolve(ix, arg)
	if err != nil {
		return err
	}
	if err := notDeleted(ix, e); err != nil {
		return err
	}
	orig, err := s.TaskBytes(e.ULID)
	if err != nil {
		return err
	}
	// old is nil for a broken task: its file does not decode, or holds
	// another ULID.
	old, err := model.DecodeTask(orig)
	if err != nil || old.ULID != e.ULID {
		old = nil
	}
	path, edited, err := a.editFile(ops.TaskFileName(e), orig)
	if err != nil {
		return err
	}
	for {
		if bytes.Equal(edited, orig) {
			if err := os.Remove(path); err != nil {
				return err
			}
			return a.printTask(ix, e.ULID, verb(false, ""))
		}
		t, err := checkEdit(ix, e.ULID, old, edited)
		if err == nil {
			if old != nil {
				host, herr := a.writerHost()
				if herr != nil {
					return herr
				}
				addWatchers(t, newMentions(old.Body, t.Body, host)...)
			}
			var committed bool
			committed, err = a.ss.Update(s, store.Op{Name: "edit", Task: e.ULID}, func(tx *store.Tx) error {
				cur, err := s.TaskBytes(e.ULID)
				if err != nil {
					return err
				}
				if !bytes.Equal(cur, orig) {
					return errChangedWhileEditing
				}
				return tx.PutTask(t)
			})
			if err == nil {
				if err := os.Remove(path); err != nil {
					return err
				}
				return a.printTask(ix, e.ULID, verb(committed, "Updated"))
			}
			var ve *store.ValidationError
			if !errors.As(err, &ve) {
				return fmt.Errorf("%s; the edited file is kept at %s", describe(err), path)
			}
		}
		writeErr(a.stderr, err)
		if !a.askRetry() {
			return fmt.Errorf("the edit is canceled; the edited file is kept at %s", path)
		}
		if edited, err = a.reopen(path, edited); err != nil {
			return err
		}
	}
}

// checkEdit decodes the edited file of task ulid and refuses the changes
// edit does not make. old is the task before the edit, nil when it was
// broken; then only the ulid is checked. The workspace can be changed only
// when the old one is missing or not a workspace of ix, to repair it.
func checkEdit(ix *index.Index, ulid model.ULID, old *model.Task, edited []byte) (*model.Task, error) {
	t, err := model.DecodeTask(edited)
	if err != nil {
		return nil, fmt.Errorf("the file does not decode: %w", err)
	}
	if old == nil {
		if t.ULID != ulid {
			return nil, fmt.Errorf("the ulid must stay %s", ulid)
		}
		return t, nil
	}
	fixed := func(key string) error { return fmt.Errorf("%s cannot be changed", key) }
	moved := func(key string) error { return fmt.Errorf("%s cannot be changed here; use hedgehug mv", key) }
	switch {
	case t.ULID != old.ULID:
		return nil, fixed("ulid")
	case t.ID != old.ID:
		return nil, fixed("id")
	case !t.Created.Equal(old.Created):
		return nil, fixed("created")
	case !t.Updated.Equal(old.Updated):
		return nil, fixed("updated")
	case !sameTime(t.DeletedAt, old.DeletedAt):
		return nil, fixed("deleted_at")
	case !sameULID(t.Parent, old.Parent):
		return nil, moved("parent")
	case t.Rank != old.Rank:
		return nil, moved("rank")
	case !slices.Equal(t.FormerIDs, old.FormerIDs):
		return nil, fixed("former_ids")
	case t.Workspace != old.Workspace && hasWorkspace(ix, old.Workspace):
		return nil, fmt.Errorf("workspace cannot be changed here; use hedgehug mv --workspace")
	}
	return t, nil
}

// hasWorkspace reports whether ulid is a workspace of ix.
func hasWorkspace(ix *index.Index, ulid model.ULID) bool {
	_, ok := ix.Workspace(ulid)
	return ok
}

func sameTime(a, b *time.Time) bool {
	return a == nil && b == nil || a != nil && b != nil && a.Equal(*b)
}

func sameULID(a, b *model.ULID) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}
