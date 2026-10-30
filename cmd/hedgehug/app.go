package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// app holds the global flags and what the commands share.
type app struct {
	*env
	storeName string
	jsonOut   bool
	noColor   bool
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
	root.AddCommand(a.storeCmd(), a.addCmd(), a.lsCmd(), a.showCmd(), a.setCmd(), a.statusCmd(), a.doneCmd(), a.depCmd(), a.rmCmd(), a.restoreCmd(), a.purgeCmd(), a.mvCmd(), a.editCmd(), a.commentCmd(), a.logCmd(), a.schemaCmd(), a.doctorCmd(), a.workspaceCmd())
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

// update runs one operation as one commit, with the author from the
// environment. committed is false when the operation changed nothing.
func (a *app) update(s *store.Store, op store.Op, fn func(*store.Tx) error) (bool, error) {
	author, err := config.Author(a.getenv)
	if err != nil {
		return false, err
	}
	op.Author = author
	beforeUpdate()
	return s.Update(op, fn)
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
	id, err := ix.Resolve(arg)
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
	e, _ := ix.Task(id)
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
