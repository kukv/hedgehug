package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/usecase/reports"
)

// barWidth is how many columns the longest bar of a report takes.
const barWidth = 24

// bar draws v as a share of top.
func bar(v, top float64) string {
	if top <= 0 || v <= 0 {
		return ""
	}
	return strings.Repeat("█", max(1, int(v/top*barWidth+0.5)))
}

func num(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

func (a *app) reportCmd() *cobra.Command {
	var workspace string
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Print reports read from the history",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.ErrOrStderr())
			_ = cmd.Usage()
			return usageErrorf("a subcommand is needed")
		},
	}
	cmd.PersistentFlags().StringVar(&workspace, "workspace", "", "the workspace (default: the one of hedgehug use)")

	var by string
	burndown := &cobra.Command{
		Use:   "burndown [sprint]",
		Short: "What a sprint holds and has left, day by day",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := ""
			if len(args) > 0 {
				name = args[0]
			}
			return a.reportBurndown(name, workspace, by)
		},
	}
	burndown.Flags().StringVar(&by, "by", "", "points or count (default: points)")

	var last string
	velocity := &cobra.Command{
		Use:   "velocity",
		Short: "What the closed sprints finished",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			n, err := reports.ParseLast(last)
			if err != nil {
				return err
			}
			return a.reportVelocity(workspace, n)
		},
	}
	velocity.Flags().StringVar(&last, "last", "", "how many sprints (default: 10)")

	var since, until, cfdBy string
	cfd := &cobra.Command{
		Use:   "cfd",
		Short: "How many tasks each category holds, day by day",
		Args:  cobra.NoArgs,
		RunE:  func(_ *cobra.Command, _ []string) error { return a.reportCFD(workspace, since, until, cfdBy) },
	}
	cfd.Flags().StringVar(&since, "since", "", "the first day (default: 30 days ago)")
	cfd.Flags().StringVar(&until, "until", "", "the last day (default: today)")
	cfd.Flags().StringVar(&cfdBy, "by", "", "category or status (default: category)")

	cycle := &cobra.Command{
		Use:   "cycle",
		Short: "How long finished tasks took",
		Args:  cobra.NoArgs,
		RunE:  func(_ *cobra.Command, _ []string) error { return a.reportCycle(workspace, since, until) },
	}
	cycle.Flags().StringVar(&since, "since", "", "the first day (default: 30 days ago)")
	cycle.Flags().StringVar(&until, "until", "", "the last day (default: today)")

	var sprint string
	estimate := &cobra.Command{
		Use:   "estimate",
		Short: "The estimates against the time logged",
		Args:  cobra.NoArgs,
		RunE:  func(_ *cobra.Command, _ []string) error { return a.reportEstimate(workspace, sprint) },
	}
	estimate.Flags().StringVar(&sprint, "sprint", "", "only the tasks of this sprint")

	cmd.AddCommand(burndown, velocity, cfd, cycle, estimate)
	return cmd
}

func (a *app) reportBurndown(name, workspace, by string) error {
	v, err := reports.Burndown(a.ss, name, workspace, by)
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(v)
	}
	if _, err := fmt.Fprintf(a.stdout, "%s (%s), by %s\n", v.Sprint.Name, v.Sprint.State, v.By); err != nil {
		return err
	}
	if v.Note != "" {
		if _, err := fmt.Fprintln(a.stdout, v.Note); err != nil {
			return err
		}
	}
	top := 0.0
	for _, d := range v.Days {
		top = max(top, d.Remaining)
	}
	rows := make([][]string, len(v.Days))
	for i, d := range v.Days {
		rows[i] = []string{d.Date, num(d.Scope), num(d.Done), num(d.Remaining), num(d.Added), bar(d.Remaining, top)}
	}
	return writeTable(a.stdout, []string{"DATE", "SCOPE", "DONE", "LEFT", "ADDED", ""}, rows, nil)
}

func (a *app) reportVelocity(workspace string, last int) error {
	vs, err := reports.Velocity(a.ss, workspace, last)
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(vs)
	}
	top := 0.0
	for _, v := range vs {
		top = max(top, v.Points)
	}
	rows := make([][]string, len(vs))
	for i, v := range vs {
		rows[i] = []string{v.Sprint.Name, num(v.Points), fmt.Sprint(v.Tasks), fmt.Sprint(v.Unpointed), bar(v.Points, top)}
	}
	return writeTable(a.stdout, []string{"SPRINT", "POINTS", "TASKS", "NO POINTS", ""}, rows, nil)
}

func (a *app) reportCFD(workspace, since, until, by string) error {
	v, err := reports.CFD(a.ss, workspace, since, until, by)
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(v)
	}
	header := append([]string{"DATE"}, upper(v.Columns)...)
	rows := make([][]string, len(v.Days))
	for i, d := range v.Days {
		row := []string{d.Date}
		for _, c := range v.Columns {
			row = append(row, fmt.Sprint(d.Counts[c]))
		}
		rows[i] = row
	}
	return writeTable(a.stdout, header, rows, nil)
}

func upper(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = strings.ToUpper(s)
	}
	return out
}

func (a *app) reportCycle(workspace, since, until string) error {
	v, err := reports.Cycle(a.ss, workspace, since, until)
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(v)
	}
	if _, err := fmt.Fprintf(a.stdout, "%s to %s\n", v.Since, v.Until); err != nil {
		return err
	}
	if err := writeTable(a.stdout, []string{"", "TASKS", "MEDIAN", "P85"}, [][]string{
		{"cycle", fmt.Sprint(v.Cycle.Count), v.Cycle.Median, v.Cycle.P85},
		{"lead", fmt.Sprint(v.Lead.Count), v.Lead.Median, v.Lead.P85},
	}, nil); err != nil {
		return err
	}
	for _, w := range v.Warnings {
		a.ss.AddWarning(w)
	}
	return nil
}

func (a *app) reportEstimate(workspace, sprint string) error {
	v, err := reports.Estimate(a.ss, workspace, sprint)
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(v)
	}
	rows := make([][]string, 0, len(v.Tasks)+1)
	for _, t := range v.Tasks {
		rows = append(rows, []string{t.ID, t.Estimate, t.Actual, t.Title})
	}
	rows = append(rows, []string{"total", v.Estimate, v.Actual, ""})
	return writeTable(a.stdout, []string{"ID", "ESTIMATE", "ACTUAL", "TITLE"}, rows, nil)
}
