package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/usecase/views"
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
	var layout, query, groupBy, kanbanColumns, swimlane, ganttStart, ganttEnd string
	var roadmap bool
	var columns []string
	var wip map[string]int
	save := &cobra.Command{
		Use:   "save <name>",
		Short: "Save a view, or change the settings given of a saved one",
		Args:  exactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			var ch views.ViewChange
			str := func(flag string, v *string) *string {
				if c.Flags().Changed(flag) {
					return v
				}
				return nil
			}
			ch.Layout, ch.Query, ch.GroupBy = str("layout", &layout), str("query", &query), str("group-by", &groupBy)
			ch.KanbanColumns, ch.Swimlane = str("kanban-columns", &kanbanColumns), str("swimlane", &swimlane)
			ch.GanttStart, ch.GanttEnd = str("gantt-start", &ganttStart), str("gantt-end", &ganttEnd)
			if c.Flags().Changed("roadmap") {
				ch.Roadmap = &roadmap
			}
			if c.Flags().Changed("columns") {
				ch.Columns = &columns
			}
			if c.Flags().Changed("wip") {
				ch.WIP = &wip
			}
			return a.viewSave(args[0], ch)
		},
	}
	f := save.Flags()
	f.StringVar(&layout, "layout", "", "tree, kanban, table or gantt")
	f.StringVar(&query, "query", "", "the query of the view")
	f.StringSliceVar(&columns, "columns", nil, "the columns of the table, such as id,title,status,due")
	f.StringVar(&groupBy, "group-by", "", "what the table groups by: none, parent, status, assignee or project")
	f.StringVar(&kanbanColumns, "kanban-columns", "", "the columns of the kanban: status or category")
	f.StringVar(&swimlane, "swimlane", "", "the rows of the kanban: none, assignee or project")
	f.StringToIntVar(&wip, "wip", nil, "the limits of the kanban columns, such as in_progress=3; replaces the ones saved, and an empty value takes them off")
	f.StringVar(&ganttStart, "gantt-start", "", "the date property the bars of the gantt start on")
	f.StringVar(&ganttEnd, "gantt-end", "", "the date property the bars of the gantt end on")
	f.BoolVar(&roadmap, "roadmap", false, "show projects and epics only on the gantt")
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
			Long: "List the tasks of a saved view.\n\n" +
				"--store names the store the view is saved in. The query runs on every\n" +
				"store, as the GUI shows a view; store: in the query narrows it.",
			Args: exactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error { return a.viewShow(args[0]) },
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

func (a *app) viewSave(name string, ch views.ViewChange) error {
	r, err := views.ViewSave(a.ss, name, ch)
	if err != nil {
		return err
	}
	line := fmt.Sprintf("Saved view %s (%s)", name, r.View.Layout)
	if !r.Committed {
		line = fmt.Sprintf("No changes: view %s is already saved with these settings", name)
	}
	return a.printView(r, line)
}

func (a *app) viewRm(name string) error {
	r, err := views.ViewRemove(a.ss, name)
	if err != nil {
		return err
	}
	return a.printView(r, "Removed view "+name)
}

func (a *app) viewRename(oldName, newName string) error {
	r, err := views.ViewRename(a.ss, oldName, newName)
	if err != nil {
		return err
	}
	return a.printView(r, fmt.Sprintf("Renamed view %s to %s", oldName, newName))
}

func (a *app) viewList() error {
	vs, err := views.ViewList(a.ss)
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
	v, err := views.ViewGet(a.ss, name)
	if err != nil {
		return err
	}
	// --store says where the view is read from. The tasks are the query's, on
	// every store, as the GUI shows a view; store: in the query narrows it.
	own := a.ss
	a.ss = own.ForStore("")
	defer func() { a.ss = own }()
	if v.Layout == "tree" {
		return a.lsTree(v.Query, false, nil)
	}
	return a.ls(v.Query, false, nil)
}

// printView prints the view r wrote as JSON, or line as text.
func (a *app) printView(r views.ViewResult, line string) error {
	if a.jsonOut {
		return a.printJSON(r.Write())
	}
	_, err := fmt.Fprintln(a.stdout, line)
	return err
}
