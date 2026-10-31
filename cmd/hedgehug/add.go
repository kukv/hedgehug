package main

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/ops"
)

func (a *app) addCmd() *cobra.Command {
	var parent, kind, status, workspace string
	var props []string
	cmd := &cobra.Command{
		Use:   "add <title>",
		Short: "Add a task",
		Args:  minArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			assigns, err := ops.ParseAssigns(props)
			if err != nil {
				return err
			}
			return a.add(strings.Join(args, " "), parent, workspace, kind, status, assigns)
		},
	}
	f := cmd.Flags()
	f.StringVar(&parent, "parent", "", "the parent task")
	f.StringVar(&kind, "kind", "", "the kind (default: the schema's default kind)")
	f.StringVar(&status, "status", "", "the status (default: the default status of the todo category)")
	f.StringVar(&workspace, "workspace", "", "the workspace (default: the parent's, or the only one)")
	f.StringArrayVarP(&props, "prop", "p", nil, "set a field or a property, as in hedgehug set (repeatable)")
	return cmd
}

func (a *app) add(title, parentArg, workspaceArg, kind, status string, assigns []ops.Assign) error {
	r, err := a.ss.Add(ops.AddInput{Title: title, Parent: parentArg, Workspace: workspaceArg, Kind: kind, Status: status, Assigns: assigns})
	if err != nil {
		return err
	}
	return a.printTask(r.Index, r.ULID, verb(r.Committed, "Added"))
}
