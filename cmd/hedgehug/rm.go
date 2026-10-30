package main

import (
	"bufio"
	"errors"
	"fmt"
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
	return &cobra.Command{
		Use:   "restore <id>",
		Short: "Restore a deleted task and what the same rm deleted under it",
		Args:  exactArgs(1),
		RunE:  func(_ *cobra.Command, args []string) error { return a.restore(args[0]) },
	}
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

// descendants returns the tasks under id for which follow is true, going
// down only through those, depth first in rank order. A parent cycle stops
// the walk.
func descendants(ix *index.Index, id model.ID, follow func(*index.Entry) bool) []*index.Entry {
	seen := map[model.ID]bool{id: true}
	var out []*index.Entry
	var walk func(model.ID)
	walk = func(p model.ID) {
		for _, c := range ix.Children(p) {
			if seen[c.ID] || !follow(c) {
				continue
			}
			seen[c.ID] = true
			out = append(out, c)
			walk(c.ID)
		}
	}
	walk(id)
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
	if e.DeletedAt != nil {
		return fmt.Errorf("%s is already deleted", refName(ix, e.ID))
	}
	targets := []*index.Entry{e}
	for _, d := range descendants(ix, e.ID, func(*index.Entry) bool { return true }) {
		if d.DeletedAt == nil {
			targets = append(targets, d)
		}
	}
	now := a.now().Truncate(time.Second)
	if _, err := a.update(s, store.Op{Name: "rm", Task: e.ID}, func(tx *store.Tx) error {
		for _, x := range targets {
			t, err := tx.Task(x.ID)
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

// notDeleted refuses to change the task e when it is deleted, and points
// to restore.
func notDeleted(ix *index.Index, e *index.Entry) error {
	if e.DeletedAt != nil {
		return fmt.Errorf("%s is deleted; restore %s first", refName(ix, e.ID), taskLabel(e))
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

func (a *app) restore(arg string) error {
	s, ix, err := a.open()
	if err != nil {
		return err
	}
	e, err := a.resolve(ix, arg)
	if err != nil {
		return err
	}
	if e.DeletedAt == nil {
		return fmt.Errorf("%s is not deleted", refName(ix, e.ID))
	}
	if e.Parent != nil {
		if p, ok := ix.Task(*e.Parent); ok {
			if err := liveParent(ix, p); err != nil {
				return err
			}
		}
	}
	at := *e.DeletedAt
	targets := append([]*index.Entry{e}, descendants(ix, e.ID, func(d *index.Entry) bool {
		return d.DeletedAt != nil && d.DeletedAt.Equal(at)
	})...)
	if _, err := a.update(s, store.Op{Name: "restore", Task: e.ID}, func(tx *store.Tx) error {
		for _, x := range targets {
			t, err := tx.Task(x.ID)
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
	if e.DeletedAt == nil {
		return fmt.Errorf("%s is not deleted; delete it with hedgehug rm first", refName(ix, e.ID))
	}
	targets := append([]*index.Entry{e}, descendants(ix, e.ID, func(*index.Entry) bool { return true })...)
	for _, d := range targets[1:] {
		if d.DeletedAt == nil {
			return fmt.Errorf("%s under it is not deleted", refName(ix, d.ID))
		}
	}
	if !yes {
		if err := a.confirm(ix, targets); err != nil {
			return err
		}
	}
	if _, err := a.update(s, store.Op{Name: "purge", Task: e.ID}, func(tx *store.Tx) error {
		for _, x := range targets {
			if err := tx.RemoveTask(x.ID); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return a.printAffected(ix, e, targets, "Purged", false)
}

// confirm asks on the terminal before a purge.
func (a *app) confirm(ix *index.Index, targets []*index.Entry) error {
	if !a.isTerminal(a.stdin) {
		return usageErrorf("purge removes files for good; give --yes to go ahead")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Purge %d tasks for good?\n", len(targets))
	for _, x := range targets {
		b.WriteString("  " + refName(ix, x.ID) + "\n")
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
				cur, _ = ix.Task(x.ID)
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
