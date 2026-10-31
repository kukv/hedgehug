package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/doctor"
	"github.com/kukv/hedgehug/internal/ops"
)

func (a *app) doctorCmd() *cobra.Command {
	var fix bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Find what is wrong with the files, the work tree and git of the store, and with --fix repair what a sync repairs",
		Args:  exactArgs(0),
		RunE:  func(*cobra.Command, []string) error { return a.doctor(fix) },
	}
	cmd.Flags().BoolVar(&fix, "fix", false, "repair parent cycles, children of deleted tasks, tasks in another workspace than their parent and ids that clash, in one commit; then tell what is left")
	return cmd
}

func (a *app) doctor(fix bool) error {
	r, err := a.ss.Doctor(fix)
	if err != nil {
		return err
	}
	if !fix {
		return a.printFindings(r.Findings)
	}
	if a.jsonOut {
		if r.Repaired == nil {
			r.Repaired = []ops.Fixed{}
		}
		if r.Findings == nil {
			r.Findings = []doctor.Finding{}
		}
		if err := a.printJSON(r); err != nil {
			return err
		}
		if len(r.Findings) > 0 {
			return errFindings
		}
		return nil
	}
	if len(r.Repaired) == 0 {
		if _, err := fmt.Fprintln(a.stdout, "Nothing to repair"); err != nil {
			return err
		}
	}
	for _, x := range r.Repaired {
		if _, err := fmt.Fprintln(a.stdout, x.Line); err != nil {
			return err
		}
	}
	return a.printFindings(r.Findings)
}
