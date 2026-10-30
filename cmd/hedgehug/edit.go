package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/model"
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
	s, ix, err := a.open()
	if err != nil {
		return err
	}
	e, err := a.resolve(ix, arg)
	if err != nil {
		return err
	}
	if err := notDeleted(ix, e); err != nil {
		return err
	}
	orig, err := s.TaskBytes(e.ID)
	if err != nil {
		return err
	}
	// old is nil for a broken task: its file does not decode, or holds
	// another id.
	old, err := model.DecodeTask(orig)
	if err != nil || old.ID != e.ID {
		old = nil
	}
	path, edited, err := a.editFile(taskFileName(e), orig)
	if err != nil {
		return err
	}
	for {
		if bytes.Equal(edited, orig) {
			if err := os.Remove(path); err != nil {
				return err
			}
			return a.printTask(ix, e.ID, verb(false, ""))
		}
		t, err := checkEdit(e.ID, old, edited)
		if err == nil {
			var committed bool
			committed, err = a.update(s, store.Op{Name: "edit", Task: e.ID}, func(tx *store.Tx) error {
				cur, err := s.TaskBytes(e.ID)
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
				return a.printTask(ix, e.ID, verb(committed, "Updated"))
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

// checkEdit decodes the edited file of task id and refuses the changes
// edit does not make. old is the task before the edit, nil when it was
// broken; then only the id is checked.
func checkEdit(id model.ID, old *model.Task, edited []byte) (*model.Task, error) {
	t, err := model.DecodeTask(edited)
	if err != nil {
		return nil, fmt.Errorf("the file does not decode: %w", err)
	}
	if old == nil {
		if t.ID != id {
			return nil, fmt.Errorf("the id must stay %s", id)
		}
		return t, nil
	}
	fixed := func(key string) error { return fmt.Errorf("%s cannot be changed", key) }
	moved := func(key string) error { return fmt.Errorf("%s cannot be changed here; use hedgehug mv", key) }
	switch {
	case t.ID != old.ID:
		return nil, fixed("id")
	case t.Key != old.Key:
		return nil, fixed("key")
	case !t.Created.Equal(old.Created):
		return nil, fixed("created")
	case !t.Updated.Equal(old.Updated):
		return nil, fixed("updated")
	case !sameTime(t.DeletedAt, old.DeletedAt):
		return nil, fixed("deleted_at")
	case !sameID(t.Parent, old.Parent):
		return nil, moved("parent")
	case t.Rank != old.Rank:
		return nil, moved("rank")
	case !slices.Equal(t.FormerKeys, old.FormerKeys):
		return nil, fixed("former_keys")
	case t.Workspace != old.Workspace:
		return nil, fmt.Errorf("workspace cannot be changed here; use hedgehug mv --workspace")
	}
	return t, nil
}

func sameTime(a, b *time.Time) bool {
	return a == nil && b == nil || a != nil && b != nil && a.Equal(*b)
}

func sameID(a, b *model.ID) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}
