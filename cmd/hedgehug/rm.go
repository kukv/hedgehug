package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
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

func (a *app) purgeCmd() *cobra.Command {
	var yes, moved, rewrite bool
	cmd := &cobra.Command{
		Use:   "purge <id> [--yes]\n  hedgehug purge --moved [--store <store>] [--yes]\n  hedgehug purge --rewrite-history <id> [--store <store>]",
		Short: "Remove the files of a deleted task and the tasks under it, for good; or the forward files of moved tasks; or the tasks from the git history",
		Args: func(cmd *cobra.Command, args []string) error {
			if moved && rewrite {
				return usageErrorf("--moved and --rewrite-history do not go together")
			}
			if moved {
				if len(args) > 0 {
					return usageErrorf("--moved takes no task id")
				}
				return nil
			}
			return exactArgs(1)(cmd, args)
		},
		RunE: func(_ *cobra.Command, args []string) error {
			if moved {
				return a.purgeMoved(yes)
			}
			if rewrite {
				return a.purgeRewriteHistory(args[0])
			}
			return a.purge(args[0], yes)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "do not ask (not for --rewrite-history, which asks for the store name on a terminal only)")
	cmd.Flags().BoolVar(&rewrite, "rewrite-history", false, "remove the task and the tasks under it from the git history of its store, and force push it to every remote")
	cmd.Flags().BoolVar(&moved, "moved", false, "remove the forward files that moves to other stores left in the store")
	return cmd
}

func (a *app) rm(arg string) error {
	res, err := a.ss.Remove(arg)
	if err != nil {
		return err
	}
	return a.printAffected(res.Index, res.Entry, res.Targets, "Deleted", true)
}

func (a *app) restore(arg string) error {
	res, err := a.ss.Restore(arg)
	if err != nil {
		return err
	}
	return a.printAffected(res.Index, res.Entry, res.Targets, "Restored", true)
}

func (a *app) purge(arg string, yes bool) error {
	// Without --yes the targets are listed and confirmed first; Purge then
	// refuses when they changed. With it, PurgeUnchecked works from the tasks
	// as they are under the lock.
	var res ops.AffectedResult
	var err error
	if yes {
		res, err = a.ss.PurgeUnchecked(arg)
	} else {
		var ix *index.Index
		var targets []*index.Entry
		ix, _, targets, err = a.ss.PurgeTargets(arg)
		if err != nil {
			return err
		}
		if err = a.confirm(ix, targets); err != nil {
			return err
		}
		confirmed := make([]model.ULID, len(targets))
		for i, x := range targets {
			confirmed[i] = x.ULID
		}
		res, err = a.ss.Purge(arg, confirmed)
	}
	if err != nil {
		return err
	}
	return a.printAffected(res.Index, res.Entry, res.Targets, "Purged", false)
}

// confirm asks on the terminal before a purge.
func (a *app) confirm(ix *index.Index, targets []*index.Entry) error {
	if !a.isTerminal(a.stdin) {
		return usageErrorf("purge removes files for good; give --yes to go ahead")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Purge %d tasks for good?\n", len(targets))
	for _, x := range targets {
		b.WriteString("  " + ops.RefName(ix, x.ULID) + "\n")
	}
	b.WriteString("[y/N] ")
	if _, err := fmt.Fprint(a.stderr, b.String()); err != nil {
		return err
	}
	line, err := a.readLine()
	if err != nil && line == "" {
		return errors.New("purge canceled")
	}
	if ans := strings.ToLower(strings.TrimSpace(line)); ans != "y" && ans != "yes" {
		return errors.New("purge canceled")
	}
	return nil
}

// printAffected prints the tasks a write changed: as JSON (read again
// after the write when refresh is true, as they were otherwise), or as one
// line about the task named.
func (a *app) printAffected(ix *index.Index, e *index.Entry, targets []*index.Entry, verb string, refresh bool) error {
	if a.jsonOut {
		if refresh {
			if err := ix.Refresh(); err != nil {
				return err
			}
		}
		out := make([]*ops.TaskView, 0, len(targets))
		for _, x := range targets {
			cur := x
			if refresh {
				cur, _ = ix.Task(x.ULID)
			}
			out = append(out, ops.TaskViewOf(ix, cur))
		}
		return a.printJSON(out)
	}
	line := fmt.Sprintf("%s %s %s", verb, a.style().id(ops.TaskLabel(ix, e)), e.Title)
	switch n := len(targets) - 1; n {
	case 0:
	case 1:
		line += " and 1 task under it"
	default:
		line += fmt.Sprintf(" and %d tasks under it", n)
	}
	_, err := fmt.Fprintln(a.stdout, line)
	return err
}

// forwardLine is a forward file as purge --moved lists it (D20):
// "01JBZ…  API-12 → org-b WEB-45".
func forwardLine(v ops.ForwardView) string {
	line := v.ULID.String() + "  "
	if v.ID != "" {
		line += v.ID + " "
	}
	line += "→ "
	if v.ToStore == "" {
		return line + v.ToULID.String()
	}
	line += v.ToStore
	if v.ToID != "" {
		line += " " + v.ToID
	}
	return line
}

// forwardFiles is "1 forward file", "2 forward files".
func forwardFiles(n int) string {
	if n == 1 {
		return "1 forward file"
	}
	return fmt.Sprintf("%d forward files", n)
}

// purgeMoved removes the forward files of the store: on a terminal after the
// list and a question, else only with --yes.
func (a *app) purgeMoved(yes bool) error {
	list, err := a.ss.PurgeMovedList()
	if err != nil {
		return err
	}
	if len(list) == 0 {
		return a.printPurgedMoved(nil, "")
	}
	if !yes {
		if !a.isTerminal(a.stdin) {
			// the same refusal the operation gives
			_, _, err := a.ss.PurgeMoved(false)
			return err
		}
		var b strings.Builder
		for _, v := range list {
			b.WriteString(forwardLine(v) + "\n")
		}
		fmt.Fprintf(&b, "Remove %s? [y/N] ", forwardFiles(len(list)))
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
	}
	var removed []ops.ForwardView
	var commit string
	if yes {
		removed, commit, err = a.ss.PurgeMoved(true)
	} else {
		// what the user said yes to, and nothing that came in since
		confirmed := make([]model.ULID, len(list))
		for i, v := range list {
			confirmed[i] = v.ULID
		}
		removed, commit, err = a.ss.PurgeMovedConfirmed(confirmed)
	}
	if err != nil {
		return err
	}
	return a.printPurgedMoved(removed, commit)
}

// printPurgedMoved prints what purge --moved removed: a line, or JSON.
func (a *app) printPurgedMoved(removed []ops.ForwardView, commit string) error {
	if a.jsonOut {
		if removed == nil {
			removed = []ops.ForwardView{}
		}
		return a.printJSON(struct {
			Removed []ops.ForwardView `json:"removed"`
			Commit  string            `json:"commit,omitempty"`
		}{removed, commit})
	}
	if len(removed) == 0 {
		_, err := fmt.Fprintln(a.stdout, "No forward files")
		return err
	}
	_, err := fmt.Fprintf(a.stdout, "Removed %s\n", forwardFiles(len(removed)))
	return err
}
