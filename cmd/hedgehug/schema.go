package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/doctor"
	"github.com/kukv/hedgehug/internal/ops"
)

// errFindings is what a command that found problems returns: the problems
// are printed, so run exits 1 without a message of its own.
var errFindings = ops.ErrFindings

func (a *app) schemaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Work on the schema of the store",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.ErrOrStderr())
			_ = cmd.Usage()
			return usageErrorf("a subcommand is needed")
		},
	}
	cmd.AddCommand(a.checkCmd(), a.migrateCmd())
	return cmd
}

func (a *app) checkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Find where the schema does not fit the store and its tasks",
		Args:  exactArgs(0),
		RunE: func(*cobra.Command, []string) error {
			fs, err := a.ss.SchemaCheck()
			if err != nil {
				return err
			}
			return a.printFindings(fs)
		},
	}
}

// printFindings prints what a check found. It returns errFindings when there
// is anything to report.
func (a *app) printFindings(fs []doctor.Finding) error {
	if a.jsonOut {
		if fs == nil {
			fs = []doctor.Finding{}
		}
		if err := a.printJSON(fs); err != nil {
			return err
		}
	} else {
		if len(fs) == 0 {
			_, err := fmt.Fprintln(a.stdout, "No problems")
			return err
		}
		for _, f := range fs {
			_, _ = fmt.Fprintf(a.stdout, "%s: %s\n", f.Where, f.Msg)
			if f.Hint != "" {
				_, _ = fmt.Fprintf(a.stdout, "  hint: %s\n", f.Hint)
			}
		}
		_, _ = fmt.Fprintf(a.stdout, "%d problems\n", len(fs))
	}
	if len(fs) > 0 {
		return errFindings
	}
	return nil
}
