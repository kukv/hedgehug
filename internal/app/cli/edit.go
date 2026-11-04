package cli

import (
	"bytes"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/lib/osfs"
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
	es, err := tasks.BeginEdit(a.ss, arg)
	if err != nil {
		return err
	}
	path, edited, err := a.editFile(es.FileName(), es.Bytes)
	if err != nil {
		return err
	}
	for {
		if bytes.Equal(edited, es.Bytes) {
			if err := osfs.Remove(path); err != nil {
				return err
			}
			return a.printTask(es.Unchanged(), verb(false, ""))
		}
		t, err := es.Check(edited)
		if err == nil {
			if err := tasks.PrepareEdit(a.ss, es, t); err != nil {
				return err
			}
			var r usecase.TaskResult
			r, err = tasks.WriteEdit(a.ss, es, t)
			if err == nil {
				if err := osfs.Remove(path); err != nil {
					return err
				}
				return a.printTask(r, verb(r.Committed, "Updated"))
			}
			if _, ok := usecase.ViolationsOf(err); !ok {
				return fmt.Errorf("%s; the edited file is kept at %s", usecase.Describe(err), path)
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
