package main

import (
	"github.com/spf13/cobra"
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
	r, err := a.ss.SetStatus(arg, name)
	if err != nil {
		return err
	}
	return a.printTask(r.Index, r.ULID, verb(r.Committed, "Updated"))
}
