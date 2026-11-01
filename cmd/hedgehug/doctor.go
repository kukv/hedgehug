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
	rs, err := a.ss.Doctor(fix)
	if err != nil {
		return err
	}
	failing := false
	for i, r := range rs {
		failing = failing || doctor.Failing(r.Findings)
		if a.jsonOut {
			if r.Repaired == nil {
				rs[i].Repaired = []ops.Fixed{}
			}
			if r.Findings == nil {
				rs[i].Findings = []doctor.Finding{}
			}
			continue
		}
		if err := a.writeDoctor(r, fix, len(rs) > 1, i == 0); err != nil {
			return err
		}
	}
	if a.jsonOut {
		if err := a.printJSON(rs); err != nil {
			return err
		}
	}
	if failing {
		return errFindings
	}
	return nil
}

// writeDoctor writes the text of one store's report: under a heading with
// the store's name when there are several stores (a blank line apart), the
// repairs when --fix asked for them, and the findings.
func (a *app) writeDoctor(r ops.DoctorReport, fix, heading, first bool) error {
	if heading {
		if !first {
			if _, err := fmt.Fprintln(a.stdout); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(a.stdout, "== %s\n", r.Store); err != nil {
			return err
		}
	}
	if fix {
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
	}
	return a.writeFindings(r.Findings)
}
