package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/view"
)

func (a *app) viewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view",
		Short: "Manage the saved views of a store",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.ErrOrStderr())
			_ = cmd.Usage()
			return usageErrorf("a subcommand is needed")
		},
	}
	var layout, query, groupBy, kanbanColumns, swimlane string
	var columns []string
	save := &cobra.Command{
		Use:   "save <name>",
		Short: "Save a view, or change the settings given of a saved one",
		Args:  exactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			var ch ops.ViewChange
			str := func(flag string, v *string) *string {
				if c.Flags().Changed(flag) {
					return v
				}
				return nil
			}
			ch.Layout, ch.Query, ch.GroupBy = str("layout", &layout), str("query", &query), str("group-by", &groupBy)
			ch.KanbanColumns, ch.Swimlane = str("kanban-columns", &kanbanColumns), str("swimlane", &swimlane)
			if c.Flags().Changed("columns") {
				ch.Columns = &columns
			}
			return a.viewSave(args[0], ch)
		},
	}
	f := save.Flags()
	f.StringVar(&layout, "layout", "", "tree, kanban or table")
	f.StringVar(&query, "query", "", "the query of the view")
	f.StringSliceVar(&columns, "columns", nil, "the columns of the table, such as id,title,status,due")
	f.StringVar(&groupBy, "group-by", "", "what the table groups by: none, parent, status, assignee or project")
	f.StringVar(&kanbanColumns, "kanban-columns", "", "the columns of the kanban: status or category")
	f.StringVar(&swimlane, "swimlane", "", "the rows of the kanban: none, assignee or project")
	cmd.AddCommand(save,
		&cobra.Command{
			Use:   "list",
			Short: "List the saved views",
			Args:  cobra.NoArgs,
			RunE:  func(_ *cobra.Command, _ []string) error { return a.viewList() },
		},
		&cobra.Command{
			Use:   "show <name>",
			Short: "List the tasks of a saved view",
			Args:  exactArgs(1),
			RunE:  func(_ *cobra.Command, args []string) error { return a.viewShow(args[0]) },
		},
		&cobra.Command{
			Use:   "rm <name>",
			Short: "Remove a saved view",
			Args:  exactArgs(1),
			RunE:  func(_ *cobra.Command, args []string) error { return a.viewRm(args[0]) },
		},
		&cobra.Command{
			Use:   "rename <old> <new>",
			Short: "Rename a saved view",
			Args:  exactArgs(2),
			RunE:  func(_ *cobra.Command, args []string) error { return a.viewRename(args[0], args[1]) },
		},
	)
	return cmd
}

func (a *app) viewSave(name string, ch ops.ViewChange) error {
	r, err := a.ss.ViewSave(name, ch)
	if err != nil {
		return err
	}
	line := fmt.Sprintf("Saved view %s (%s)", name, r.View.Layout)
	if !r.Committed {
		line = fmt.Sprintf("%s: view %s is already saved so", verb(false, ""), name)
	}
	return a.printView(r.View, line)
}

func (a *app) viewRm(name string) error {
	r, err := a.ss.ViewRemove(name)
	if err != nil {
		return err
	}
	return a.printView(r.View, "Removed view "+name)
}

func (a *app) viewRename(oldName, newName string) error {
	r, err := a.ss.ViewRename(oldName, newName)
	if err != nil {
		return err
	}
	return a.printView(r.View, fmt.Sprintf("Renamed view %s to %s", oldName, newName))
}

func (a *app) viewList() error {
	vs, err := a.ss.ViewList()
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(vs)
	}
	rows := make([][]string, len(vs))
	for i, v := range vs {
		q := v.Query
		if q == "" {
			q = "-"
		}
		rows[i] = []string{v.Name, v.Layout, q}
	}
	return writeTable(a.stdout, []string{"NAME", "LAYOUT", "QUERY"}, rows, nil)
}

func (a *app) viewShow(name string) error {
	v, err := a.ss.ViewGet(name)
	if err != nil {
		return err
	}
	q, err := ops.ParseQuery(v.Query)
	if err != nil {
		return err
	}
	if v.Layout == "tree" {
		return a.lsTree(q, false, nil)
	}
	return a.ls(q, false, nil)
}

// printView prints v as JSON, or line as text.
func (a *app) printView(v *view.View, line string) error {
	if a.jsonOut {
		return a.printJSON(ops.SavedViewOf(v))
	}
	_, err := fmt.Fprintln(a.stdout, line)
	return err
}
