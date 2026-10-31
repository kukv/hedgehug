package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// app holds the global flags and what the commands share.
type app struct {
	*env
	ss      *ops.Session
	jsonOut bool
	noColor bool
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
	pf.StringVar(&a.ss.StoreName, "store", "", "the store to work on, by name")
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

// afterWrite is Session.Update's step after a commit, for the one write that
// does not go through Update: the repair of doctor --fix.
func (a *app) afterWrite(s *store.Store) {
	if err := sync.MarkPending(s, a.now()); err != nil {
		a.ss.Warn("the write is not marked for sync: " + err.Error())
		return
	}
	_, e, err := a.ss.Entry()
	if err != nil {
		return
	}
	a.ss.AfterWrite(s, e)
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
	cfg, _, err := a.ss.Config()
	if err != nil {
		return
	}
	targets, err := a.ss.SyncTargets(cfg, false)
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
