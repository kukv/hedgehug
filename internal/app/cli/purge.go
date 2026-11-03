package cli

import (
	"cmp"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/lib/plural"
)

func (a *app) purgeCmd() *cobra.Command {
	var olderThan string
	var yes bool
	cmd := &cobra.Command{
		Use:   "purge [--older-than <N>d] [--store <name>] [--yes]",
		Short: "Remove, for good, the deleted tasks and the forward files of moved tasks that are older than a time",
		Long: "Purge removes the files of the tasks deleted more than N days ago, with their comments and work logs, and the forward files that moves to other stores left more than N days ago. " +
			"N is 90 unless --older-than says; 0d is everything.\n\n" +
			"A deleted task with a task under it that is not deleted stays, and purge tells of it.",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				return usageErrorf("purge takes no task id; it removes by age: hedgehug purge --older-than <N>d (0d is everything)")
			}
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			days, err := usecase.ParseOlderThan(olderThan)
			if err != nil {
				return err
			}
			return a.purge(days, yes)
		},
	}
	cmd.Flags().StringVar(&olderThan, "older-than", fmt.Sprintf("%dd", usecase.DefaultPurgeDays), "remove what is older than this many days, such as 90d (0d for everything)")
	cmd.Flags().BoolVar(&yes, "yes", false, "do not ask")
	return cmd
}

// purge lists what is to go and, on a terminal, asks; off a terminal it goes
// only with --yes.
func (a *app) purge(days int, yes bool) error {
	plan, err := a.ss.PurgeList(days)
	if err != nil {
		return err
	}
	if plan.Empty() {
		return a.printPurged(usecase.PurgeResult{PurgePlan: plan})
	}
	var res usecase.PurgeResult
	switch {
	case yes:
		res, err = a.ss.Purge(days, true)
	case !a.isTerminal(a.stdin):
		res, err = a.ss.Purge(days, false) // the refusal the operation gives
	default:
		if err = a.confirmPurge(plan); err == nil {
			res, err = a.ss.PurgeConfirmed(days, plan)
		}
	}
	if err != nil {
		return err
	}
	return a.printPurged(res)
}

// confirmPurge shows what is to go and asks.
func (a *app) confirmPurge(p usecase.PurgePlan) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s:\n", p.Store)
	for _, t := range p.Tasks {
		fmt.Fprintf(&b, "  %s  %s\n", cmp.Or(t.ID, t.ULID.String()), t.Title)
	}
	if len(p.Forwards) > 0 {
		fmt.Fprintf(&b, "  %s\n", plural.Count(len(p.Forwards), "forward file"))
	}
	fmt.Fprintf(&b, "Purge %s and %s from %s for good? [y/N] ", plural.Count(len(p.Tasks), "task"), plural.Count(len(p.Forwards), "forward file"), p.Store)
	if _, err := fmt.Fprint(a.stderr, b.String()); err != nil {
		return err
	}
	line, err := a.readLine()
	if err != nil && line == "" {
		return errors.New("Canceled")
	}
	if ans := strings.ToLower(strings.TrimSpace(line)); ans != "y" && ans != "yes" {
		return errors.New("Canceled")
	}
	return nil
}

// printPurged prints what purge removed: a line, or JSON.
func (a *app) printPurged(r usecase.PurgeResult) error {
	if a.jsonOut {
		return a.printJSON(r)
	}
	if r.Empty() {
		_, err := fmt.Fprintln(a.stdout, "Nothing to purge")
		return err
	}
	_, err := fmt.Fprintf(a.stdout, "Purged %s and %s from %s\n", plural.Count(len(r.Tasks), "task"), plural.Count(len(r.Forwards), "forward file"), r.Store)
	return err
}
