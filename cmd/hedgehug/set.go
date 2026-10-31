package main

import (
	"slices"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/store"
)

func (a *app) setCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <id> <key=value>...",
		Short: "Change fields and properties of a task",
		Long: `Change fields and properties of a task.

  key=value   set (a list becomes just value)
  key=        clear
  key+=value  add to a list
  key-=value  remove from a list

Fields: title, kind, status, assignees, watchers, labels, repos, blocked_by.
Any other key is a property of the schema. Use hedgehug mv for the parent
and the order.`,
		Args: minArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			assigns, err := parseAssigns(args[1:])
			if err != nil {
				return err
			}
			return a.set(args[0], assigns)
		},
	}
}

func (a *app) set(arg string, assigns []assign) error {
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
	committed, err := a.ss.Update(s, store.Op{Name: "set", Task: e.ULID}, func(tx *store.Tx) error {
		t, err := tx.Task(e.ULID)
		if err != nil {
			return err
		}
		assignees := slices.Clone(t.Assignees)
		for _, as := range assigns {
			if err := applyAssign(t, as, ix.Schema(), a.ss.ResolveULID(ix)); err != nil {
				return err
			}
		}
		watchNewAssignees(t, assignees)
		return tx.PutTask(t)
	})
	if err != nil {
		return err
	}
	return a.printTask(ix, e.ULID, verb(committed, "Updated"))
}
