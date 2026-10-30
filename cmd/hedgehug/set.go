package main

import (
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
	s, ix, err := a.open()
	if err != nil {
		return err
	}
	e, err := a.resolve(ix, arg)
	if err != nil {
		return err
	}
	committed, err := a.update(s, store.Op{Name: "set", Task: e.ID}, func(tx *store.Tx) error {
		t, err := tx.Task(e.ID)
		if err != nil {
			return err
		}
		for _, as := range assigns {
			if err := applyAssign(t, as, ix.Schema(), a.resolveID(ix)); err != nil {
				return err
			}
		}
		return tx.PutTask(t)
	})
	if err != nil {
		return err
	}
	return a.printTask(ix, e.ID, verb(committed, "Updated"))
}
