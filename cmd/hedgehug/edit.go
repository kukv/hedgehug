package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/store"
)

func (a *app) editCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit the file of a task in $EDITOR",
		Args:  exactArgs(1),
		RunE:  func(_ *cobra.Command, args []string) error { return a.edit(args[0]) },
	}
}

func (a *app) edit(arg string) error {
	es, err := a.ss.BeginEdit(arg)
	if err != nil {
		return err
	}
	path, edited, err := a.editFile(ops.TaskFileName(es.Entry), es.Bytes)
	if err != nil {
		return err
	}
	for {
		if bytes.Equal(edited, es.Bytes) {
			if err := os.Remove(path); err != nil {
				return err
			}
			return a.printTask(es.Index, es.Entry.ULID, verb(false, ""))
		}
		t, err := ops.CheckEdit(es.Index, es.Entry.ULID, es.Old, edited)
		if err == nil {
			if err := a.ss.PrepareEdit(es, t); err != nil {
				return err
			}
			var r ops.TaskResult
			r, err = a.ss.WriteEdit(es, t)
			if err == nil {
				if err := os.Remove(path); err != nil {
					return err
				}
				return a.printTask(r.Index, r.ULID, verb(r.Committed, "Updated"))
			}
			var ve *store.ValidationError
			if !errors.As(err, &ve) {
				return fmt.Errorf("%s; the edited file is kept at %s", describe(err), path)
			}
		}
		writeErr(a.stderr, err)
		if !a.askRetry() {
			return fmt.Errorf("the edit is canceled; the edited file is kept at %s", path)
		}
		if edited, err = a.reopen(path, edited); err != nil {
			return err
		}
	}
}
