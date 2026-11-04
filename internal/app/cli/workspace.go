package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/usecase/workspaces"
	"github.com/kukv/hedgehug/internal/lib/plural"
)

func (a *app) workspaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Manage the workspaces of a store",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.ErrOrStderr())
			_ = cmd.Usage()
			return usageErrorf("a subcommand is needed")
		},
	}
	var prefix string
	add := &cobra.Command{
		Use:   "add <name> --prefix <P>",
		Short: "Add a workspace",
		Args:  exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if prefix == "" {
				return usageErrorf("--prefix is required: the id prefix of the workspace, such as WEB")
			}
			return a.workspaceAdd(args[0], prefix)
		},
	}
	add.Flags().StringVar(&prefix, "prefix", "", "the id prefix of the workspace, such as WEB")
	cmd.AddCommand(add,
		&cobra.Command{
			Use:   "ls",
			Short: "List the workspaces",
			Args:  cobra.NoArgs,
			RunE:  func(_ *cobra.Command, _ []string) error { return a.workspaceLs() },
		},
		&cobra.Command{
			Use:   "rename <workspace> <name>",
			Short: "Rename a workspace",
			Args:  exactArgs(2),
			RunE:  func(_ *cobra.Command, args []string) error { return a.workspaceRename(args[0], args[1]) },
		},
		&cobra.Command{
			Use:   "set-prefix <workspace> <P>",
			Short: "Change the id prefix of a workspace and renumber its tasks",
			Args:  exactArgs(2),
			RunE:  func(_ *cobra.Command, args []string) error { return a.workspacePrefix(args[0], args[1]) },
		},
		&cobra.Command{
			Use:   "rm <workspace>",
			Short: "Remove a workspace that has no tasks",
			Args:  exactArgs(1),
			RunE:  func(_ *cobra.Command, args []string) error { return a.workspaceRm(args[0]) },
		},
	)
	return cmd
}

func (a *app) workspaceAdd(name, prefix string) error {
	r, err := workspaces.WorkspaceAdd(a.ss, name, prefix)
	if err != nil {
		return err
	}
	w := r.View()
	return a.printWorkspace(r, fmt.Sprintf("Added workspace %s (%s, %s)", w.Name, w.IDPrefix, w.ULID))
}

func (a *app) workspaceRename(arg, name string) error {
	r, err := workspaces.WorkspaceRename(a.ss, arg, name)
	if err != nil {
		return err
	}
	w := r.View()
	return a.printWorkspace(r, fmt.Sprintf("Renamed workspace %s to %s (%s, %s)", r.OldName, w.Name, w.IDPrefix, w.ULID))
}

func (a *app) workspacePrefix(arg, prefix string) error {
	r, err := workspaces.WorkspacePrefix(a.ss, arg, prefix)
	if err != nil {
		return err
	}
	w := r.View()
	if !r.Committed {
		return a.printWorkspace(r, fmt.Sprintf("%s: %s already has the id prefix %s (%s)", verb(false, ""), w.Name, w.IDPrefix, w.ULID))
	}
	if a.jsonOut {
		return a.printWorkspace(r, "")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Changed the id prefix of %s to %s (%s); renumbered %s\n", w.Name, w.IDPrefix, w.ULID, plural.Count(r.Renumbered, "task"))
	for _, c := range r.Renumbering() {
		fmt.Fprintf(&b, "  %s → %s\n", c.Old, c.New)
	}
	_, err = fmt.Fprint(a.stdout, b.String())
	return err
}

func (a *app) workspaceRm(arg string) error {
	r, err := workspaces.WorkspaceRemove(a.ss, arg)
	if err != nil {
		return err
	}
	w := r.View()
	return a.printWorkspace(r, fmt.Sprintf("Removed workspace %s (%s, %s)", w.Name, w.IDPrefix, w.ULID))
}

func (a *app) workspaceLs() error {
	wss, err := workspaces.WorkspaceList(a.ss)
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(wss)
	}
	rows := make([][]string, len(wss))
	for i, w := range wss {
		rows[i] = []string{w.Name, w.IDPrefix, w.ULID, fmt.Sprint(w.Tasks)}
	}
	return writeTable(a.stdout, []string{"NAME", "PREFIX", "ULID", "TASKS"}, rows, nil)
}

// printWorkspace prints the workspace r wrote as JSON, or line as text.
func (a *app) printWorkspace(r workspaces.WorkspaceResult, line string) error {
	if a.jsonOut {
		return a.printJSON(r.Write())
	}
	_, err := fmt.Fprintln(a.stdout, line)
	return err
}
