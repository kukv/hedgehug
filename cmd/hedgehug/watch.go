package main

import (
	"github.com/spf13/cobra"
)

func (a *app) watchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "watch <id>",
		Short: "Watch a task",
		Args:  exactArgs(1),
		RunE:  func(_ *cobra.Command, args []string) error { return a.watch(args[0], true) },
	}
}

func (a *app) unwatchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unwatch <id>",
		Short: "Stop watching a task",
		Args:  exactArgs(1),
		RunE:  func(_ *cobra.Command, args []string) error { return a.watch(args[0], false) },
	}
}

// watch adds the writer of the store to the watchers of the task arg
// names, or takes it out.
func (a *app) watch(arg string, on bool) error {
	r, err := a.ss.Watch(arg, on)
	if err != nil {
		return err
	}
	return a.printTask(r.Index, r.ULID, verb(r.Committed, "Updated"))
}
