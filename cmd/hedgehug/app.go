package main

import (
	"bufio"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

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
	in      *bufio.Reader // the one buffered reader over stdin; see readLine
}

// readLine reads a line of stdin. Every prompt reads through the one reader,
// so that a buffered second answer is not lost to a reader of its own.
func (a *app) readLine() (string, error) {
	if a.in == nil {
		a.in = bufio.NewReader(a.stdin)
	}
	return a.in.ReadString('\n')
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
	root.AddCommand(a.storeCmd(), a.addCmd(), a.lsCmd(), a.showCmd(), a.setCmd(), a.statusCmd(), a.doneCmd(), a.depCmd(), a.rmCmd(), a.restoreCmd(), a.undoCmd(), a.purgeCmd(), a.mvCmd(), a.editCmd(), a.commentCmd(), a.logCmd(), a.schemaCmd(), a.doctorCmd(), a.workspaceCmd(), a.viewCmd(), a.syncCmd(), a.inboxCmd(), a.watchCmd(), a.unwatchCmd(), a.mergeDriverCmd(), a.authCmd(), a.useCmd())
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

// beforeCommand reports the failure of a background sync that the user has
// not been told of, and starts one for each store with a remote that is due
// (ops.BeforeCommand), except in the sync itself and in a merge driver.
func (a *app) beforeCommand(cmd *cobra.Command, _ []string) {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Parent() != nil && c.Parent().Parent() == nil && (c.Name() == "sync" || c.Name() == "merge-driver") {
			return // the sync itself, and a merge in the middle of one
		}
	}
	a.ss.BeforeCommand(
		func(s *store.Store, name string) error { return sync.Start(s, name, a.getenv) },
		func(name, msg string) {
			_, _ = fmt.Fprintf(a.stderr, "hedgehug: background sync of %s failed: %s (run hedgehug sync)\n", name, msg)
		})
}

// verb is the heading of a command that printed what it wrote: heading, or
// "No changes" when nothing was committed.
func verb(committed bool, heading string) string {
	if !committed {
		return "No changes"
	}
	return heading
}

// describe turns err into the text after "hedgehug: ", with the violations
// of a refused write on the lines after it.
func describe(err error) string {
	var ve *store.ValidationError
	if errors.As(err, &ve) {
		return ops.ViolationsText(ve)
	}
	if errors.Is(err, store.ErrLocked) {
		return "another hedgehug is writing to the store; try again"
	}
	return err.Error()
}
