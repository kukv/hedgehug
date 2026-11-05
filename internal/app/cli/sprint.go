package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/cli/prompt"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/planning"
	"github.com/kukv/hedgehug/internal/lib/plural"
)

func (a *app) sprintCmd() *cobra.Command {
	var workspace string
	cmd := &cobra.Command{
		Use:   "sprint",
		Short: "Manage the sprints of a workspace",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.ErrOrStderr())
			_ = cmd.Usage()
			return usageErrorf("a subcommand is needed")
		},
	}
	cmd.PersistentFlags().StringVar(&workspace, "workspace", "", "the workspace (default: the one of hedgehug use)")

	var start, end, goal string
	add := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a planned sprint",
		Args:  exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			w, err := planning.SprintAdd(a.ss, planning.SprintAddInput{Name: args[0], Start: start, End: end, Goal: goal, Workspace: workspace})
			if err != nil {
				return err
			}
			return a.printSprint(w, "Added sprint "+w.Sprint.Name)
		},
	}
	add.Flags().StringVar(&start, "start", "", "the first day (YYYY-MM-DD)")
	add.Flags().StringVar(&end, "end", "", "the last day (YYYY-MM-DD)")
	add.Flags().StringVar(&goal, "goal", "", "what the sprint is for")

	var state string
	ls := &cobra.Command{
		Use:   "ls",
		Short: "List the sprints",
		Args:  cobra.NoArgs,
		RunE:  func(_ *cobra.Command, _ []string) error { return a.sprintLs(workspace, state) },
	}
	ls.Flags().StringVar(&state, "state", "", "only this state: planned, active or closed")

	var to string
	closeCmd := &cobra.Command{
		Use:   "close <name>",
		Short: "Close an active sprint and move its unfinished tasks",
		Args:  exactArgs(1),
		RunE:  func(_ *cobra.Command, args []string) error { return a.sprintClose(args[0], workspace, to) },
	}
	closeCmd.Flags().StringVar(&to, "to", "", "where the unfinished tasks go: next, backlog or the name of a sprint")

	cmd.AddCommand(add, ls, closeCmd,
		&cobra.Command{
			Use:   "set <name> <key=value>...",
			Short: "Change the name, start, end or goal of a sprint",
			Args:  minArgs(2),
			RunE: func(_ *cobra.Command, args []string) error {
				assigns, err := usecase.ParseAssigns(args[1:])
				if err != nil {
					return err
				}
				w, err := planning.SprintSet(a.ss, args[0], workspace, assigns)
				if err != nil {
					return err
				}
				return a.printSprint(w, "Updated sprint "+w.Sprint.Name)
			},
		},
		&cobra.Command{
			Use:   "start <name>",
			Short: "Start a planned sprint",
			Args:  exactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				w, err := planning.SprintStart(a.ss, args[0], workspace)
				if err != nil {
					return err
				}
				return a.printSprint(w, fmt.Sprintf("Started sprint %s (%s to %s)", w.Sprint.Name, w.Sprint.Start, w.Sprint.End))
			},
		},
		&cobra.Command{
			Use:   "rm <name>",
			Short: "Delete a sprint and take it off its tasks",
			Args:  exactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				w, err := planning.SprintRemove(a.ss, args[0], workspace)
				if err != nil {
					return err
				}
				return a.printSprint(w, "Removed sprint "+w.Sprint.Name)
			},
		},
	)
	return cmd
}

func (a *app) sprintLs(workspace, state string) error {
	vs, err := planning.SprintList(a.ss, workspace, state)
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(vs)
	}
	rows := make([][]string, len(vs))
	for i, v := range vs {
		rows[i] = []string{v.Name, v.Workspace.Name, v.State, v.Start, v.End, fmt.Sprintf("%d/%d", v.Done, v.Tasks), strconv.FormatFloat(v.Points, 'f', -1, 64)}
	}
	return writeTable(a.stdout, []string{"NAME", "WORKSPACE", "STATE", "START", "END", "DONE", "POINTS"}, rows, nil)
}

func (a *app) sprintClose(name, workspace, to string) error {
	if to == "" && a.ask().Interactive() {
		n, err := planning.SprintOpenTasks(a.ss, name, workspace)
		if err != nil {
			return err
		}
		if n > 0 {
			if to, err = a.ask().Choose(prompt.Choose{
				Question: fmt.Sprintf("%s unfinished; move to ", plural.Count(n, "task")),
				Options:  []string{planning.ToNext, planning.ToBacklog},
				ByWord:   true,
				Canceled: "canceled",
			}); err != nil {
				return err
			}
		}
	}
	w, err := planning.SprintClose(a.ss, name, workspace, to)
	if err != nil {
		return err
	}
	line := "Closed sprint " + w.Sprint.Name
	if w.Moved > 0 {
		line += fmt.Sprintf("; moved %s to %s", plural.Count(w.Moved, "task"), w.To)
	}
	return a.printSprint(w, line)
}

// printSprint prints the sprint w wrote as JSON, or line as text.
func (a *app) printSprint(w planning.SprintWrite, line string) error {
	if a.jsonOut {
		return a.printJSON(w)
	}
	_, err := fmt.Fprintln(a.stdout, line)
	return err
}
