package main

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

func (a *app) depCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "dep", Short: "Manage what blocks a task", Args: cobra.NoArgs}
	for _, add := range []bool{true, false} {
		var blockedBy string
		use, short := "add <id> --blocked-by <id>", "Make a task blocked by another"
		if !add {
			use, short = "rm <id> --blocked-by <id>", "Stop a task being blocked by another"
		}
		sub := &cobra.Command{
			Use:   use,
			Short: short,
			Args:  exactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				if blockedBy == "" {
					return usageErrorf("--blocked-by is needed")
				}
				return a.dep(args[0], blockedBy, add)
			},
		}
		sub.Flags().StringVar(&blockedBy, "blocked-by", "", "the blocking task")
		cmd.AddCommand(sub)
	}
	return cmd
}

func (a *app) dep(arg, blockerArg string, add bool) error {
	s, ix, err := a.open()
	if err != nil {
		return err
	}
	e, err := a.resolve(ix, arg)
	if err != nil {
		return err
	}
	b, err := a.resolve(ix, blockerArg)
	if err != nil {
		return err
	}
	if err := a.update(s, "dep", e.ID, func(tx *store.Tx) error {
		t, err := tx.Task(e.ID)
		if err != nil {
			return err
		}
		has := slices.Contains(t.BlockedBy, b.ID)
		switch {
		case add && has:
			return fmt.Errorf("%s is already blocked by %s", refName(ix, e.ID), refName(ix, b.ID))
		case add:
			t.BlockedBy = append(t.BlockedBy, b.ID)
		case !has:
			return fmt.Errorf("%s is not blocked by %s", refName(ix, e.ID), refName(ix, b.ID))
		default:
			t.BlockedBy = slices.DeleteFunc(t.BlockedBy, func(x model.ID) bool { return x == b.ID })
		}
		return tx.PutTask(t)
	}); err != nil {
		return err
	}
	return a.printTask(ix, e.ID, "Updated")
}
