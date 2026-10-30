package main

import (
	"fmt"

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
	root.AddCommand(a.storeCmd(), a.lsCmd())
	return root
}

// exactArgs and minArgs are cobra's, with the error as a usageError.
func exactArgs(n int) cobra.PositionalArgs { return asUsage(cobra.ExactArgs(n)) }
func minArgs(n int) cobra.PositionalArgs   { return asUsage(cobra.MinimumNArgs(n)) } //nolint:unused // used by the commands that follow in phase 1d-1

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

// update runs one operation, named name and about task, as one commit.
func (a *app) update(s *store.Store, name string, task model.ID, fn func(*store.Tx) error) error { //nolint:unused // used by the commands that follow in phase 1d-1
	author, err := config.Author(a.getenv)
	if err != nil {
		return err
	}
	return s.Update(store.Op{Name: name, Task: task, Author: author}, fn)
}

// resolve finds the task arg names.
func (a *app) resolve(ix *index.Index, arg string) (*index.Entry, error) { //nolint:unused // used by the commands that follow in phase 1d-1
	id, err := ix.Resolve(arg)
	if err != nil {
		return nil, err
	}
	e, _ := ix.Task(id)
	return e, nil
}

// describe turns err into the text after "hedgehug: ". Task 6 makes it
// list violations and candidates.
func describe(err error) string {
	return err.Error()
}
