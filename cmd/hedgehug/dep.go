package main

import (
	"github.com/spf13/cobra"
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
	r, err := a.ss.Dep(arg, blockerArg, add)
	if err != nil {
		return err
	}
	return a.printTask(r.Index, r.ULID, verb(r.Committed, "Updated"))
}
