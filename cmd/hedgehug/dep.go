package main

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/store"
)

func (a *app) depCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dep",
		Short: "Manage what blocks a task",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.ErrOrStderr())
			_ = cmd.Usage()
			return usageErrorf("a subcommand is needed")
		},
	}
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
	b, err := a.ss.Resolve(ix, blockerArg)
	if err != nil {
		return err
	}
	committed, err := a.ss.Update(s, store.Op{Name: "dep", Task: e.ULID}, func(tx *store.Tx) error {
		t, err := tx.Task(e.ULID)
		if err != nil {
			return err
		}
		has := slices.Contains(t.BlockedBy, b.ULID)
		switch {
		case add && has:
			return fmt.Errorf("%s is already blocked by %s", ops.RefName(ix, e.ULID), ops.RefName(ix, b.ULID))
		case add:
			t.BlockedBy = append(t.BlockedBy, b.ULID)
		case !has:
			return fmt.Errorf("%s is not blocked by %s", ops.RefName(ix, e.ULID), ops.RefName(ix, b.ULID))
		default:
			t.BlockedBy = slices.DeleteFunc(t.BlockedBy, func(x model.ULID) bool { return x == b.ULID })
		}
		return tx.PutTask(t)
	})
	if err != nil {
		return err
	}
	return a.printTask(ix, e.ULID, verb(committed, "Updated"))
}
