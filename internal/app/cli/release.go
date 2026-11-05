package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/planning"
)

func (a *app) releaseCmd() *cobra.Command {
	var workspace string
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Manage the releases of a workspace",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.ErrOrStderr())
			_ = cmd.Usage()
			return usageErrorf("a subcommand is needed")
		},
	}
	cmd.PersistentFlags().StringVar(&workspace, "workspace", "", "the workspace (default: the one of hedgehug use)")

	var date string
	add := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a release",
		Args:  exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			w, err := planning.ReleaseAdd(a.ss, args[0], date, workspace)
			if err != nil {
				return err
			}
			return a.printRelease(w, "Added release "+w.Release.Name)
		},
	}
	add.Flags().StringVar(&date, "date", "", "the planned date (YYYY-MM-DD)")

	var state string
	ls := &cobra.Command{
		Use:   "ls",
		Short: "List the releases",
		Args:  cobra.NoArgs,
		RunE:  func(_ *cobra.Command, _ []string) error { return a.releaseLs(workspace, state) },
	}
	ls.Flags().StringVar(&state, "state", "", "only this state: unreleased, released or archived")

	cmd.AddCommand(add, ls,
		&cobra.Command{
			Use:   "set <name> <key=value>...",
			Short: "Change the name, date or state of a release",
			Args:  minArgs(2),
			RunE: func(_ *cobra.Command, args []string) error {
				assigns, err := usecase.ParseAssigns(args[1:])
				if err != nil {
					return err
				}
				w, err := planning.ReleaseSet(a.ss, args[0], workspace, assigns)
				if err != nil {
					return err
				}
				return a.printRelease(w, "Updated release "+w.Release.Name)
			},
		},
		&cobra.Command{
			Use:   "notes <name>",
			Short: "Print the release notes in Markdown",
			Args:  exactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				notes, err := planning.ReleaseNotes(a.ss, args[0], workspace)
				if err != nil {
					return err
				}
				if a.jsonOut {
					return a.printJSON(map[string]string{"notes": notes})
				}
				_, err = fmt.Fprint(a.stdout, notes)
				return err
			},
		},
		&cobra.Command{
			Use:   "rm <name>",
			Short: "Delete a release and take it off its tasks",
			Args:  exactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				w, err := planning.ReleaseRemove(a.ss, args[0], workspace)
				if err != nil {
					return err
				}
				return a.printRelease(w, "Removed release "+w.Release.Name)
			},
		},
	)
	return cmd
}

func (a *app) releaseLs(workspace, state string) error {
	vs, err := planning.ReleaseList(a.ss, workspace, state)
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(vs)
	}
	rows := make([][]string, len(vs))
	for i, v := range vs {
		rows[i] = []string{v.Name, v.Workspace.Name, v.State, v.Date, fmt.Sprintf("%d/%d", v.Done, v.Tasks)}
	}
	return writeTable(a.stdout, []string{"NAME", "WORKSPACE", "STATE", "DATE", "DONE"}, rows, nil)
}

// printRelease prints the release w wrote as JSON, or line as text.
func (a *app) printRelease(w planning.ReleaseWrite, line string) error {
	if a.jsonOut {
		return a.printJSON(w)
	}
	_, err := fmt.Fprintln(a.stdout, line)
	return err
}
