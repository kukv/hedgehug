package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/usecase/worklogs"
)

func (a *app) worklogCmd() *cobra.Command {
	var date string
	cmd := &cobra.Command{
		Use:   "worklog <id> <time> [note...]",
		Short: "Record time spent on a task",
		Long: `Record time spent on a task, as 1h30m, 45m or 2h.

  hedgehug worklog ls <id>            list the entries
  hedgehug worklog rm <id> <entry>    delete an entry of yours`,
		Args: minArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			w, err := worklogs.Add(a.ss, args[0], args[1], date, strings.Join(args[2:], " "))
			if err != nil {
				return err
			}
			return a.printWorklog(w, fmt.Sprintf("Logged %s on %s (%s)", w.Worklog.Duration, args[0], w.Worklog.Date))
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "the day of the work (YYYY-MM-DD; default: today)")
	cmd.AddCommand(
		&cobra.Command{
			Use:   "ls <id>",
			Short: "List the time spent on a task",
			Args:  exactArgs(1),
			RunE:  func(_ *cobra.Command, args []string) error { return a.worklogLs(args[0]) },
		},
		&cobra.Command{
			Use:   "rm <id> <entry>",
			Short: "Delete an entry of yours",
			Args:  exactArgs(2),
			RunE: func(_ *cobra.Command, args []string) error {
				w, err := worklogs.Remove(a.ss, args[0], args[1])
				if err != nil {
					return err
				}
				return a.printWorklog(w, fmt.Sprintf("Removed %s from %s", w.Worklog.Duration, args[0]))
			},
		},
	)
	return cmd
}

func (a *app) worklogLs(arg string) error {
	vs, err := worklogs.List(a.ss, arg)
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(vs)
	}
	rows := make([][]string, len(vs))
	for i, v := range vs {
		rows[i] = []string{v.Date, v.Duration, v.Author, v.ULID, v.Body}
	}
	return writeTable(a.stdout, []string{"DATE", "TIME", "AUTHOR", "ULID", "NOTE"}, rows, nil)
}

// printWorklog prints the entry w wrote as JSON, or line as text.
func (a *app) printWorklog(w worklogs.WorklogWrite, line string) error {
	if a.jsonOut {
		return a.printJSON(w)
	}
	_, err := fmt.Fprintln(a.stdout, line)
	return err
}
