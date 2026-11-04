package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/lib/plural"
)

func (a *app) rmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id>",
		Short: "Delete a task and the tasks under it (they can be restored)",
		Args:  exactArgs(1),
		RunE:  func(_ *cobra.Command, args []string) error { return a.rm(args[0]) },
	}
}

func (a *app) restoreCmd() *cobra.Command {
	var at string
	cmd := &cobra.Command{
		Use:   "restore <id> [--at <time>]",
		Short: "Restore a deleted task and what the same rm deleted under it, or a task as it was at a time",
		Args:  exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if at != "" {
				return a.restoreAt(args[0], at)
			}
			return a.restore(args[0])
		},
	}
	cmd.Flags().StringVar(&at, "at", "", "put the task back as it was at this time (RFC 3339 or a date, the end of that day) or commit; its id and workspace stay")
	return cmd
}

func (a *app) rm(arg string) error {
	res, err := tasks.Remove(a.ss, arg)
	if err != nil {
		return err
	}
	return a.printAffected(res, "Deleted")
}

func (a *app) restore(arg string) error {
	res, err := tasks.Restore(a.ss, arg)
	if err != nil {
		return err
	}
	return a.printAffected(res, "Restored")
}

// printAffected prints the tasks a write changed, read again after the
// write: as JSON, or as one line about the task named.
func (a *app) printAffected(r tasks.AffectedResult, verb string) error {
	w, err := r.Write()
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(w)
	}
	line := fmt.Sprintf("%s %s %s", verb, a.style().id(w.Tasks[0].Label()), w.Tasks[0].Title)
	if n := len(w.Tasks) - 1; n > 0 {
		line += " and " + plural.Count(n, "task") + " under it"
	}
	_, err = fmt.Fprintln(a.stdout, line)
	return err
}
