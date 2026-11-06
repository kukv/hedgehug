package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/replication"
	"github.com/kukv/hedgehug/internal/lib/plural"
)

func (a *app) syncCmd() *cobra.Command {
	var status, background bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync the stores with their remotes",
		Long:  "Sync fetches, merges, repairs and pushes. It syncs the store given with --store, or every store that has a remote.",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if status {
				return a.syncStatus()
			}
			if background {
				return a.syncBackground()
			}
			return a.sync()
		},
	}
	cmd.Flags().BoolVar(&status, "status", false, "show how the syncs went and the commits not pushed, and sync nothing")
	cmd.Flags().BoolVar(&background, "background", false, "wait for push_interval after a write, then sync the store given with --store")
	_ = cmd.Flags().MarkHidden("background")
	return cmd
}

// syncBackground is what the detached child of a write runs: the failures
// are in sync.json and the log, not in the exit code.
func (a *app) syncBackground() error {
	return replication.SyncBackground(a.ss, a.stderr)
}

func (a *app) sync() error {
	reps, err := replication.Sync(a.ss, false, true)
	if len(reps) == 0 {
		if err != nil {
			return err
		}
		if a.jsonOut {
			return a.printJSON([]usecase.SyncReport{})
		}
		_, err := fmt.Fprintln(a.stdout, "Nothing to sync: no store has a remote")
		return err
	}
	for i := range reps {
		r := &reps[i]
		if r.Err != nil {
			r.Error = usecase.Describe(r.Err)
			continue
		}
		if !a.jsonOut {
			if err := a.printSync(*r); err != nil {
				return err
			}
		}
	}
	if a.jsonOut {
		if err := a.printJSON(reps); err != nil {
			return err
		}
	}
	return err
}

// printSync writes what a sync did as text.
func (a *app) printSync(r usecase.SyncReport) error {
	var b strings.Builder
	switch {
	case r.NoRemote && r.GitHubUpdated:
		fmt.Fprintf(&b, "Read GitHub for %s: links and gh updated (no remote, nothing to push)\n", r.Store)
	case r.NoRemote && r.GitHubRead:
		fmt.Fprintf(&b, "Read GitHub for %s: nothing new (no remote, nothing to push)\n", r.Store)
	case r.NoRemote:
		fmt.Fprintf(&b, "Nothing to sync: %s has no remote\n", r.Store)
	default:
		fmt.Fprintf(&b, "Synced %s: %s", r.Store, plural.Count(r.Merges, "merge"))
		if len(r.Pushed) > 0 {
			b.WriteString(", pushed to " + strings.Join(r.Pushed, ", "))
		}
		b.WriteString("\n")
		if r.GitHubUpdated {
			b.WriteString("  GitHub: links and gh updated\n")
		}
	}
	if r.Unpushable > 0 {
		fmt.Fprintf(&b, "  read-only: %s not pushed\n", plural.Count(r.Unpushable, "commit"))
	}
	for _, x := range r.Repaired {
		fmt.Fprintf(&b, "  %s\n", repairedLine(x))
	}
	for _, o := range r.Overwritten {
		who := "an unknown author"
		if o.Author != "" {
			who = o.Author
		}
		what := o.Path
		if o.ID != "" {
			what = o.Label
		}
		hint := ""
		if o.Notice != "" {
			hint = "; hedgehug inbox revert " + o.Notice
		}
		fmt.Fprintf(&b, "  overwritten: %s %s (%s's value was replaced%s)\n", what, o.Field, who, hint)
	}
	if n := len(r.Invalid); n > 0 {
		fmt.Fprintf(&b, "  invalid: %s now %s problems; find %s with hedgehug ls is:invalid\n",
			plural.Count(n, "task"), plural.Pick(n, "has", "have"), plural.Pick(n, "it", "them"))
	}
	if _, err := fmt.Fprint(a.stdout, b.String()); err != nil {
		return err
	}
	for _, w := range r.Warnings {
		if _, err := fmt.Fprintf(a.stderr, "warning: %s: %s\n", r.Store, w); err != nil {
			return err
		}
	}
	return nil
}

// repairedLine is the line of one repair: a new id as "ORG-2 (ULID) → ORG-3",
// the others as "repaired: ORG-4 (ULID) kind".
func repairedLine(x usecase.Repaired) string {
	if x.Line != "" {
		return x.Line
	}
	name := x.ULID
	if x.ID != "" {
		name = x.Label + " (" + x.ULID + ")"
	}
	return fmt.Sprintf("repaired: %s %s", name, x.Kind)
}

func (a *app) syncStatus() error {
	out, err := usecase.SyncStatusViews(a.ss)
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(out)
	}
	var b strings.Builder
	for _, x := range out {
		fmt.Fprintf(&b, "%s\n", x.Store)
		if x.Error != "" {
			fmt.Fprintf(&b, "  error:         %s\n", x.Error)
			continue
		}
		fmt.Fprintf(&b, "  last attempt:  %s\n", timeText(x.LastAttempt))
		fmt.Fprintf(&b, "  last success:  %s\n", timeText(x.LastSuccess))
		if x.PendingSince != nil {
			fmt.Fprintf(&b, "  pending since: %s\n", timeText(x.PendingSince))
		}
		if x.LastError != "" {
			fmt.Fprintf(&b, "  last error:    %s\n", x.LastError)
		}
		if x.ReadOnly != "" {
			fmt.Fprintf(&b, "  read-only:     %s\n", x.ReadOnly)
		}
		if x.Unpushed == nil {
			b.WriteString("  no remote\n")
		} else {
			fmt.Fprintf(&b, "  %s not pushed\n", plural.Count(*x.Unpushed, "commit"))
		}
	}
	_, err = fmt.Fprint(a.stdout, b.String())
	return err
}

func timeText(t *time.Time) string {
	if t == nil {
		return "never"
	}
	return t.Format(time.RFC3339)
}
