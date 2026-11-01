package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/ops"
)

func (a *app) mvCmd() *cobra.Command {
	var parent, before, after, workspace string
	var root bool
	cmd := &cobra.Command{
		Use:   "mv <id> [--workspace <ws>] [--parent <id> | --root] [--before <id> | --after <id>]",
		Short: "Move a task under another parent or among its siblings",
		Args:  exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			switch {
			case parent != "" && root:
				return usageErrorf("give --parent or --root, not both")
			case before != "" && after != "":
				return usageErrorf("give --before or --after, not both")
			case parent == "" && !root && before == "" && after == "" && workspace == "":
				return usageErrorf("say where to: --workspace, --parent, --root, --before or --after")
			}
			return a.mv(args[0], workspace, parent, root, before, after)
		},
	}
	f := cmd.Flags()
	f.StringVar(&workspace, "workspace", "", "move it with its subtree to this workspace and renumber")
	f.StringVar(&parent, "parent", "", "the new parent")
	f.BoolVar(&root, "root", false, "move to the top level")
	f.StringVar(&before, "before", "", "put it just before this sibling")
	f.StringVar(&after, "after", "", "put it just after this sibling")
	return cmd
}

func (a *app) mv(arg, workspaceArg, parentArg string, root bool, before, after string) error {
	res, err := a.ss.Move(ops.MoveInput{Arg: arg, Workspace: workspaceArg, Parent: parentArg, Root: root, Before: before, After: after})
	if err != nil {
		return err
	}
	if a.jsonOut && len(res.Detached) > 0 {
		// The task as the other commands print it, and what the move took off
		// it, as the server tells it.
		if err := res.Index.Refresh(); err != nil {
			return err
		}
		e, ok := res.Index.Task(res.ULID)
		if !ok {
			return fmt.Errorf("task %s is not in the index after the write", res.ULID)
		}
		return a.printJSON(struct {
			*ops.TaskView
			Detached []ops.Detached `json:"detached"`
		}{ops.TaskViewOf(res.Index, e), res.Detached})
	}
	if err := a.printTask(res.Index, res.ULID, verb(res.Committed, "Moved")); err != nil || a.jsonOut || !res.Moved {
		return err
	}
	var b strings.Builder
	for i, t := range res.Tasks {
		fmt.Fprintf(&b, "  %s → %s\n", res.Index.QualifyID(res.OldIDs[i]), res.Index.QualifyID(t.ID))
	}
	for _, d := range res.Detached {
		id := res.Index.QualifyID(d.ID)
		if d.Sprint != "" {
			fmt.Fprintf(&b, "  %s: removed from sprint %s\n", id, d.Sprint)
		}
		for _, r := range d.Releases {
			fmt.Fprintf(&b, "  %s: removed from release %s\n", id, r)
		}
	}
	_, err = fmt.Fprint(a.stdout, b.String())
	return err
}
