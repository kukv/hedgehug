package main

import (
	"errors"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

func (a *app) watchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "watch <id>",
		Short: "Watch a task",
		Args:  exactArgs(1),
		RunE:  func(_ *cobra.Command, args []string) error { return a.watch(args[0], true) },
	}
}

func (a *app) unwatchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unwatch <id>",
		Short: "Stop watching a task",
		Args:  exactArgs(1),
		RunE:  func(_ *cobra.Command, args []string) error { return a.watch(args[0], false) },
	}
}

var errNoAccounts = errors.New("accounts are not set up in config.yaml; add one to accounts")

// watch adds the writer of the store to the watchers of the task arg
// names, or takes it out.
func (a *app) watch(arg string, on bool) error {
	acct, ok, err := a.ss.Writer()
	if err != nil {
		return err
	}
	if !ok {
		return errNoAccounts
	}
	s, ix, err := a.ss.Open()
	if err != nil {
		return err
	}
	e, err := a.ss.Resolve(ix, arg)
	if err != nil {
		return err
	}
	if err := notDeleted(ix, e); err != nil {
		return err
	}
	name := "unwatch"
	if on {
		name = "watch"
	}
	committed, err := a.ss.Update(s, store.Op{Name: name, Task: e.ULID}, func(tx *store.Tx) error {
		t, err := tx.Task(e.ULID)
		if err != nil {
			return err
		}
		if on {
			addWatchers(t, acct.ID())
		} else {
			t.Watchers = slices.DeleteFunc(t.Watchers, func(w string) bool { return strings.EqualFold(w, acct.ID()) })
		}
		return tx.PutTask(t)
	})
	if err != nil {
		return err
	}
	return a.printTask(ix, e.ULID, verb(committed, "Updated"))
}

// addWatchers adds the accounts that t does not have yet to its watchers,
// at the end. Accounts compare without case.
func addWatchers(t *model.Task, accts ...string) {
	for _, acct := range accts {
		if !slices.ContainsFunc(t.Watchers, func(w string) bool { return strings.EqualFold(w, acct) }) {
			t.Watchers = append(t.Watchers, acct)
		}
	}
}

// mentionedAccounts returns the accounts body mentions as account IDs: a
// @login gets the host of the writer, a @host/login is as written. Without
// a writer host a @login names no account and is left out.
func mentionedAccounts(body string, writerHost string) []string {
	var out []string
	for _, m := range index.Mentions(body) {
		switch {
		case strings.Contains(m, "/"):
			out = append(out, m)
		case writerHost != "":
			out = append(out, writerHost+"/"+m)
		}
	}
	return out
}

// newMentions returns the accounts now mentions and old did not.
func newMentions(old, now, writerHost string) []string {
	was := mentionedAccounts(old, writerHost)
	var out []string
	for _, m := range mentionedAccounts(now, writerHost) {
		if !slices.ContainsFunc(was, func(w string) bool { return strings.EqualFold(w, m) }) {
			out = append(out, m)
		}
	}
	return out
}

// writerHost is the host of the account that writes to the store, "" when
// the store has none.
func (a *app) writerHost() (string, error) {
	acct, ok, err := a.ss.Writer()
	if err != nil || !ok {
		return "", err
	}
	return acct.Host, nil
}

// watchNewAssignees adds to the watchers of t the assignees it has now and
// did not in before.
func watchNewAssignees(t *model.Task, before []string) {
	for _, as := range t.Assignees {
		if !slices.ContainsFunc(before, func(b string) bool { return strings.EqualFold(b, as) }) {
			addWatchers(t, as)
		}
	}
}

// watchInTx adds accts to the watchers of the task in the operation; the
// task is not written when they all watch it already.
func watchInTx(tx *store.Tx, task model.ULID, accts ...string) error {
	if len(accts) == 0 {
		return nil
	}
	t, err := tx.Task(task)
	if err != nil {
		return err
	}
	n := len(t.Watchers)
	addWatchers(t, accts...)
	if len(t.Watchers) == n {
		return nil
	}
	return tx.PutTask(t)
}
