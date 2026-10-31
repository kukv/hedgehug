package main

import (
	"bufio"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
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
	var yes bool
	cmd := &cobra.Command{
		Use:   "purge <id>",
		Short: "Remove the files of a deleted task and the tasks under it, for good",
		Args:  exactArgs(1),
		RunE:  func(_ *cobra.Command, args []string) error { return a.purge(args[0], yes) },
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "do not ask")
	return cmd
}

// descendants returns the tasks under ulid for which follow is true, going
// down only through those, depth first in rank order. A parent cycle stops
// the walk.
func descendants(ix *index.Index, ulid model.ULID, follow func(*index.Entry) bool) []*index.Entry {
	seen := map[model.ULID]bool{ulid: true}
	var out []*index.Entry
	var walk func(model.ULID)
	walk = func(p model.ULID) {
		for _, c := range ix.Children(p) {
			if seen[c.ULID] || !follow(c) {
				continue
			}
			seen[c.ULID] = true
			out = append(out, c)
			walk(c.ULID)
		}
	}
	walk(ulid)
	return out
}

func (a *app) rm(arg string) error {
	s, ix, err := a.open()
	if err != nil {
		return err
	}
	e, err := a.resolve(ix, arg)
	if err != nil {
		return err
	}
	if err := notDeletedYet(ix, e); err != nil {
		return err
	}
	var targets []*index.Entry
	now := a.now().Truncate(time.Second)
	if _, err := a.update(s, store.Op{Name: "rm", Task: e.ULID}, func(tx *store.Tx) error {
		// Read under the lock, so a child added since is deleted too, and
		// a task deleted since is not deleted again.
		var err error
		if e, err = current(ix, e); err != nil {
			return err
		}
		if err := notDeletedYet(ix, e); err != nil {
			return err
		}
		targets = []*index.Entry{e}
		for _, d := range descendants(ix, e.ULID, func(*index.Entry) bool { return true }) {
			if d.DeletedAt == nil {
				targets = append(targets, d)
			}
		}
		for _, x := range targets {
			t, err := tx.Task(x.ULID)
			if err != nil {
				return err
			}
			t.DeletedAt = &now
			if err := tx.PutTask(t); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return a.printAffected(ix, e, targets, "Deleted", true)
}

// current refreshes ix, as a command does under the lock, and returns e as
// it is now.
func current(ix *index.Index, e *index.Entry) (*index.Entry, error) {
	if err := ix.Refresh(); err != nil {
		return nil, err
	}
	cur, ok := ix.Task(e.ULID)
	if !ok {
		return nil, fmt.Errorf("%s is no longer in the store", taskLabel(e))
	}
	return cur, nil
}

// notDeletedYet refuses to delete the task e again.
func notDeletedYet(ix *index.Index, e *index.Entry) error {
	if e.DeletedAt != nil {
		return fmt.Errorf("%s is already deleted", refName(ix, e.ULID))
	}
	return nil
}

// isDeleted refuses to restore or purge the task e when it is not deleted;
// hint ends the message.
func isDeleted(ix *index.Index, e *index.Entry, hint string) error {
	if e.DeletedAt == nil {
		return fmt.Errorf("%s is not deleted%s", refName(ix, e.ULID), hint)
	}
	return nil
}

// notDeleted refuses to change the task e when it is deleted, and points
// to restore.
func notDeleted(ix *index.Index, e *index.Entry) error {
	if e.DeletedAt != nil {
		return fmt.Errorf("%s is deleted; restore %s first", refName(ix, e.ULID), taskFileName(e))
	}
	return nil
}

// liveParent refuses a deleted task as a parent: live tasks under it would
// block its purge and the restore of its other children.
func liveParent(ix *index.Index, p *index.Entry) error {
	if err := notDeleted(ix, p); err != nil {
		return fmt.Errorf("the parent %w", err)
	}
	return nil
}

// restorable refuses to restore e when it is not deleted or its parent is.
func restorable(ix *index.Index, e *index.Entry) error {
	if err := isDeleted(ix, e, ""); err != nil {
		return err
	}
	if e.Parent != nil {
		if p, ok := ix.Task(*e.Parent); ok {
			return liveParent(ix, p)
		}
	}
	return nil
}

func (a *app) restore(arg string) error {
	s, ix, err := a.open()
	if err != nil {
		return err
	}
	e, err := a.resolve(ix, arg)
	if err != nil {
		return err
	}
	if err := restorable(ix, e); err != nil {
		return err
	}
	var targets []*index.Entry
	if _, err := a.update(s, store.Op{Name: "restore", Task: e.ULID}, func(tx *store.Tx) error {
		// Read under the lock, so a child the same rm deleted is restored
		// even when it came in after the index was read, and a task
		// restored since is refused.
		var err error
		if e, err = current(ix, e); err != nil {
			return err
		}
		if err := restorable(ix, e); err != nil {
			return err
		}
		at := *e.DeletedAt
		targets = append([]*index.Entry{e}, descendants(ix, e.ULID, func(d *index.Entry) bool {
			return d.DeletedAt != nil && d.DeletedAt.Equal(at)
		})...)
		for _, x := range targets {
			t, err := tx.Task(x.ULID)
			if err != nil {
				return err
			}
			t.DeletedAt = nil
			if err := tx.PutTask(t); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return a.printAffected(ix, e, targets, "Restored", true)
}

func (a *app) purge(arg string, yes bool) error {
	s, ix, err := a.open()
	if err != nil {
		return err
	}
	e, err := a.resolve(ix, arg)
	if err != nil {
		return err
	}
	targets, err := purgeTargets(ix, e)
	if err != nil {
		return err
	}
	if !yes {
		if err := a.confirm(ix, targets); err != nil {
			return err
		}
	}
	confirmed := targets
	if _, err := a.update(s, store.Op{Name: "purge", Task: e.ULID}, func(tx *store.Tx) error {
		// Read again under the lock: a task restored or added under it
		// since must not be purged unchecked, nor left behind.
		var err error
		if e, err = current(ix, e); err != nil {
			return err
		}
		if targets, err = purgeTargets(ix, e); err != nil {
			return err
		}
		if !yes && !sameTasks(targets, confirmed) {
			return errors.New("the tasks under it changed after the confirmation; run purge again")
		}
		for _, x := range targets {
			if err := tx.RemoveTask(x.ULID); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return a.printAffected(ix, e, targets, "Purged", false)
}

// purgeTargets returns e and every task under it, refusing when one of them
// is not deleted.
func purgeTargets(ix *index.Index, e *index.Entry) ([]*index.Entry, error) {
	if err := isDeleted(ix, e, "; delete it with hedgehug rm first"); err != nil {
		return nil, err
	}
	targets := append([]*index.Entry{e}, descendants(ix, e.ULID, func(*index.Entry) bool { return true })...)
	for _, d := range targets[1:] {
		if d.DeletedAt == nil {
			return nil, fmt.Errorf("%s under it is not deleted", refName(ix, d.ULID))
		}
	}
	return targets, nil
}

// sameTasks reports whether a and b hold the same tasks.
func sameTasks(a, b []*index.Entry) bool {
	ulids := func(es []*index.Entry) []string {
		out := make([]string, len(es))
		for i, e := range es {
			out[i] = e.ULID.String()
		}
		slices.Sort(out)
		return out
	}
	return slices.Equal(ulids(a), ulids(b))
}

// confirm asks on the terminal before a purge.
func (a *app) confirm(ix *index.Index, targets []*index.Entry) error {
	if !a.isTerminal(a.stdin) {
		return usageErrorf("purge removes files for good; give --yes to go ahead")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Purge %d tasks for good?\n", len(targets))
	for _, x := range targets {
		b.WriteString("  " + refName(ix, x.ULID) + "\n")
	}
	b.WriteString("[y/N] ")
	if _, err := fmt.Fprint(a.stderr, b.String()); err != nil {
		return err
	}
	line, err := bufio.NewReader(a.stdin).ReadString('\n')
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
		out := make([]*taskJSON, 0, len(targets))
		for _, x := range targets {
			cur := x
			if refresh {
				cur, _ = ix.Task(x.ULID)
			}
			out = append(out, entryJSON(ix, cur))
		}
		return a.printJSON(out)
	}
	line := fmt.Sprintf("%s %s %s", verb, a.style().id(taskLabel(e)), e.Title)
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
