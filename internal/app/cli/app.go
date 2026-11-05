package cli

import (
	"bufio"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/cli/prompt"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/replication"
)

// app holds the global flags and what the commands share.
type app struct {
	*env
	ss      *usecase.Session
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

// ask is the one that asks a person on the terminal. It is made at each use,
// so that the isTerminal of the env is the one in force.
func (a *app) ask() prompt.Asker {
	return prompt.Asker{Out: a.stderr, ReadLine: a.readLine, Terminal: a.isTerminal(a.stdin)}
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
		PersistentPreRunE: a.beforeRun,
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
	root.AddCommand(a.storeCmd(), a.addCmd(), a.lsCmd(), a.showCmd(), a.setCmd(), a.statusCmd(), a.doneCmd(), a.depCmd(), a.linkCmd(), a.unlinkCmd(), a.rmCmd(), a.restoreCmd(), a.undoCmd(), a.purgeCmd(), a.mvCmd(), a.editCmd(), a.commentCmd(), a.logCmd(), a.schemaCmd(), a.automationCmd(), a.doctorCmd(), a.workspaceCmd(), a.viewCmd(), a.syncCmd(), a.inboxCmd(), a.watchCmd(), a.unwatchCmd(), a.mergeDriverCmd(), a.authCmd(), a.useCmd(), a.uiCmd())
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

// noAccountCommands are the commands that run when no account is set up
// (spec §5.1): the ones that set one up, and merge-driver, which git calls in
// the middle of a sync. --help and --version never reach beforeRun.
var noAccountCommands = map[string]bool{
	"hedgehug auth login":   true,
	"hedgehug auth add":     true,
	"hedgehug auth status":  true,
	"hedgehug help":         true,
	"hedgehug merge-driver": true,
}

// errNoAccountSetUp is what a command that needs an account answers when
// there is none.
var errNoAccountSetUp = errors.New("no account is set up; add one with hedgehug auth add <name> --login <login> (or hedgehug auth login)")

// beforeRun is the hook of every command: it stops a command that needs an
// account when none is set up, then runs beforeCommand.
func (a *app) beforeRun(cmd *cobra.Command, _ []string) error {
	n, err := a.ss.AccountCount()
	if !noAccountCommands[cmd.CommandPath()] {
		if err != nil {
			return err
		}
		if n == 0 {
			return errNoAccountSetUp
		}
	}
	// With no account (or no readable config) there is no store to sync in
	// the background, and a child process would only stop at its own entry.
	if err == nil && n > 0 {
		a.beforeCommand(cmd)
	}
	return nil
}

// beforeCommand reports the failure of a background sync that the user has
// not been told of, and starts one for each store with a remote that is due
// (usecase.BeforeCommand), except in the sync itself and in a merge driver.
func (a *app) beforeCommand(cmd *cobra.Command) {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Parent() != nil && c.Parent().Parent() == nil && (c.Name() == "sync" || c.Name() == "merge-driver" || c.Name() == "ui") {
			return // the sync itself, a merge in the middle of one, and hh ui, whose server syncs
		}
	}
	replication.BeforeCommand(a.ss, func(name, msg string) {
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
