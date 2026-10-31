package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/inbox"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/query"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// app holds the global flags and what the commands share.
type app struct {
	*env
	storeName string
	jsonOut   bool
	noColor   bool

	// the writer of the store, read from config.yaml once per run
	writerDone bool
	writerAcct config.Account
	writerOK   bool
	writerErr  error
}

// usageError is a mistake in the flags or arguments; it exits 2.
type usageError struct{ error }

func (e usageError) Unwrap() error { return e.error }

func usageErrorf(format string, args ...any) error {
	return usageError{fmt.Errorf(format, args...)}
}

func (a *app) rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "hedgehug",
		Short:         "A local-first task manager on git",
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		// Every subcommand uses this one: none has a hook of its own.
		PersistentPreRun: a.beforeCommand,
		// completion comes in phase 7.
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.ErrOrStderr())
			_ = cmd.Usage()
			return usageErrorf("a command is needed")
		},
	}
	root.SetVersionTemplate("hedgehug {{.Version}}\n")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return usageError{err} })
	pf := root.PersistentFlags()
	pf.StringVar(&a.storeName, "store", "", "the store to work on, by name")
	pf.BoolVar(&a.jsonOut, "json", false, "print JSON")
	pf.BoolVar(&a.noColor, "no-color", false, "do not color the output")
	root.AddCommand(a.storeCmd(), a.addCmd(), a.lsCmd(), a.showCmd(), a.setCmd(), a.statusCmd(), a.doneCmd(), a.depCmd(), a.rmCmd(), a.restoreCmd(), a.undoCmd(), a.purgeCmd(), a.mvCmd(), a.editCmd(), a.commentCmd(), a.logCmd(), a.schemaCmd(), a.doctorCmd(), a.workspaceCmd(), a.syncCmd(), a.inboxCmd(), a.watchCmd(), a.unwatchCmd(), a.mergeDriverCmd())
	return root
}

// exactArgs and minArgs are cobra's, with the error as a usageError.
func exactArgs(n int) cobra.PositionalArgs { return asUsage(cobra.ExactArgs(n)) }
func minArgs(n int) cobra.PositionalArgs   { return asUsage(cobra.MinimumNArgs(n)) }

func asUsage(f cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := f(cmd, args); err != nil {
			return usageError{err}
		}
		return nil
	}
}

func (a *app) config() (*config.Config, config.Paths, error) {
	p, err := config.Resolve(a.getenv)
	if err != nil {
		return nil, p, err
	}
	c, err := config.Load(p)
	return c, p, err
}

// openStore opens the store the command works on.
func (a *app) openStore() (*store.Store, error) {
	c, _, err := a.config()
	if err != nil {
		return nil, err
	}
	e, err := c.Select(a.storeName)
	if err != nil {
		return nil, err
	}
	return store.Open(e.Path)
}

// open opens the store and its index.
func (a *app) open() (*store.Store, *index.Index, error) {
	s, err := a.openStore()
	if err != nil {
		return nil, nil, err
	}
	ix, err := index.Open(s)
	if err != nil {
		return nil, nil, err
	}
	return s, ix, nil
}

// beforeUpdate lets tests write to the store after a command read its index
// and before it writes, as another process could.
var beforeUpdate = func() {}

// writer returns the account that writes to the store the command works
// on; ok is false when the store has none. It reads the config once.
func (a *app) writer() (acct config.Account, ok bool, err error) {
	if !a.writerDone {
		a.writerDone = true
		a.writerAcct, a.writerOK, a.writerErr = a.loadWriter()
	}
	return a.writerAcct, a.writerOK, a.writerErr
}

func (a *app) loadWriter() (config.Account, bool, error) {
	c, _, err := a.config()
	if err != nil {
		return config.Account{}, false, err
	}
	e, err := c.Select(a.storeName)
	if err != nil {
		return config.Account{}, false, err
	}
	acct, ok := c.Writer(e)
	return acct, ok, nil
}

// accountID is the ID of the writer's account, or "" when the store has none.
func (a *app) accountID() (string, error) {
	acct, ok, err := a.writer()
	if err != nil || !ok {
		return "", err
	}
	return acct.ID(), nil
}

// update runs one operation as one commit. The author is the store's
// account, or the git config when the store has none. committed is false
// when the operation changed nothing.
func (a *app) update(s *store.Store, op store.Op, fn func(*store.Tx) error) (bool, error) {
	acct, ok, err := a.writer()
	if err != nil {
		return false, err
	}
	author, err := config.CommitAuthor(acct, ok, a.getenv)
	if err != nil {
		return false, err
	}
	op.Author = author
	if ok {
		op.Account = acct.ID()
	}
	beforeUpdate()
	committed, err := s.Update(op, fn)
	if err == nil && committed {
		a.afterWrite(s)
	}
	return committed, err
}

// afterWrite notes that the store has a commit to push and, when it has a
// remote, starts the background sync. The commit is made, so a failure here
// is only a warning.
func (a *app) afterWrite(s *store.Store) {
	if err := sync.MarkPending(s, a.now()); err != nil {
		_, _ = fmt.Fprintln(a.stderr, "warning: the write is not marked for sync:", err)
		return
	}
	c, _, err := a.config()
	if err != nil {
		return
	}
	e, err := c.Select(a.storeName)
	if err != nil || !syncsInBackground(e) {
		return
	}
	if err := sync.Start(s, e.Name, a.getenv); err != nil {
		_, _ = fmt.Fprintln(a.stderr, "warning: the background sync did not start:", err)
	}
}

// syncsInBackground is whether the store e is synced in the background: only
// a store with a remote is.
func syncsInBackground(e config.StoreEntry) bool { return len(e.Remotes) > 0 }

// beforeCommand reports the failure of a background sync that the user has
// not been told of, and starts one for each store with a remote that is due.
// What goes wrong here is left to the command itself to tell: it fails on
// the same store or config.
func (a *app) beforeCommand(cmd *cobra.Command, _ []string) {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Parent() != nil && c.Parent().Parent() == nil && (c.Name() == "sync" || c.Name() == "merge-driver") {
			return // the sync itself, and a merge in the middle of one
		}
	}
	cfg, _, err := a.config()
	if err != nil {
		return
	}
	targets, err := a.syncTargets(cfg, false)
	if err != nil {
		return
	}
	for _, e := range targets {
		if !syncsInBackground(e) {
			continue
		}
		s, err := store.Open(e.Path)
		if err != nil {
			continue
		}
		if msg, err := sync.TakeError(s); err == nil && msg != "" {
			_, _ = fmt.Fprintf(a.stderr, "hedgehug: background sync of %s failed: %s (run hedgehug sync)\n", e.Name, msg)
		}
		if st, err := sync.ReadState(s); err == nil && sync.Due(st, a.now(), cfg.Sync.Interval) {
			_ = sync.Start(s, e.Name, a.getenv)
		}
	}
}

// syncOptions is what sync.Run needs for the store e, which need not be in c
// yet: the account that writes, the accounts that are me, and the other
// stores whose id prefixes it checks.
func (a *app) syncOptions(c *config.Config, e config.StoreEntry) (sync.Options, error) {
	acct, ok := c.Writer(e)
	author, err := config.CommitAuthor(acct, ok, a.getenv)
	if err != nil {
		return sync.Options{}, err
	}
	opt := sync.Options{Remotes: e.Remotes, Author: author, Me: c.Me(), Now: a.now}
	if ok {
		opt.Account = acct.ID()
	}
	for _, o := range c.Stores {
		if o.Name != e.Name {
			opt.Others = append(opt.Others, sync.OtherStore{Name: o.Name, Path: o.Path})
		}
	}
	return opt, nil
}

// queryOptions is what the queries on the store s need besides its index: the
// accounts @me stands for, and the tasks with an unread overwritten item in
// the inbox.
func (a *app) queryOptions(s *store.Store) (query.Options, error) {
	c, _, err := a.config()
	if err != nil {
		return query.Options{}, err
	}
	items, err := inbox.Open(s).List(false)
	if err != nil {
		return query.Options{}, err
	}
	opt := query.Options{Me: c.Me(), Overwritten: map[model.ULID]bool{}}
	for _, it := range items {
		if it.Kind == inbox.Overwritten && !it.Task.IsZero() {
			opt.Overwritten[it.Task] = true
		}
	}
	return opt, nil
}

// verb is the heading of a command that printed what it wrote: heading, or
// "No changes" when nothing was committed.
func verb(committed bool, heading string) string {
	if !committed {
		return "No changes"
	}
	return heading
}

// ambiguousError is index.AmbiguousError with the candidates spelled out.
type ambiguousError struct {
	input string
	noun  string   // what the candidates are, "tasks" when empty
	lines []string // "<id> (<ULID>) <title>" per candidate
}

func (e *ambiguousError) Error() string {
	noun := e.noun
	if noun == "" {
		noun = "tasks"
	}
	return fmt.Sprintf("%q matches %d %s:\n  %s", e.input, len(e.lines), noun, strings.Join(e.lines, "\n  "))
}

// resolve finds the task arg names.
func (a *app) resolve(ix *index.Index, arg string) (*index.Entry, error) {
	ulid, err := ix.Resolve(arg)
	var ae *index.AmbiguousError
	if errors.As(err, &ae) {
		lines := make([]string, len(ae.Candidates))
		for i, c := range ae.Candidates {
			lines[i] = refName(ix, c)
		}
		return nil, &ambiguousError{input: arg, lines: lines}
	}
	if err != nil {
		return nil, err
	}
	e, _ := ix.Task(ulid)
	return e, nil
}

// resolveULID is resolve for applyAssign.
func (a *app) resolveULID(ix *index.Index) func(string) (model.ULID, error) {
	return func(s string) (model.ULID, error) {
		e, err := a.resolve(ix, s)
		if err != nil {
			return model.ULID{}, err
		}
		return e.ULID, nil
	}
}

// describe turns err into the text after "hedgehug: ", with the violations
// of a refused write on the lines after it.
func describe(err error) string {
	var ve *store.ValidationError
	if errors.As(err, &ve) {
		var b strings.Builder
		b.WriteString("the change breaks the schema:")
		for _, v := range ve.Violations {
			b.WriteString("\n  " + v.String())
		}
		return b.String()
	}
	if errors.Is(err, store.ErrLocked) {
		return "another hedgehug is writing to the store; try again"
	}
	return err.Error()
}
