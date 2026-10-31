package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

func (a *app) statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <id> <status>",
		Short: "Change the status of a task",
		Args:  exactArgs(2),
		RunE:  func(_ *cobra.Command, args []string) error { return a.setStatus(args[0], args[1]) },
	}
}

func (a *app) doneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "done <id>",
		Short: "Mark a task done (the default status of the done category)",
		Args:  exactArgs(1),
		RunE:  func(_ *cobra.Command, args []string) error { return a.setStatus(args[0], "") },
	}
}

// setStatus sets the status of the task arg names; an empty name means the
// default status of the done category.
func (a *app) setStatus(arg, name string) error {
	s, ix, err := a.ss.Open()
	if err != nil {
		return err
	}
	sc := ix.Schema()
	if name == "" {
		st, ok := sc.DefaultStatus(schema.CategoryDone)
		if !ok {
			return fmt.Errorf("the schema has no default done status")
		}
		name = st.Name
	} else if _, ok := sc.Status(name); !ok {
		names := make([]string, len(sc.Statuses))
		for i, st := range sc.Statuses {
			names[i] = st.Name
		}
		return fmt.Errorf("%q is not a status (statuses: %s)", name, strings.Join(names, ", "))
	}
	e, err := a.ss.Resolve(ix, arg)
	if err != nil {
		return err
	}
	if err := notDeleted(ix, e); err != nil {
		return err
	}
	committed, err := a.ss.Update(s, store.Op{Name: "status", Task: e.ULID}, func(tx *store.Tx) error {
		t, err := tx.Task(e.ULID)
		if err != nil {
			return err
		}
		t.Status = name
		return tx.PutTask(t)
	})
	if err != nil {
		return err
	}
	return a.printTask(ix, e.ULID, verb(committed, "Updated"))
}
