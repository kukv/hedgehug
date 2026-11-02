package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/doctor"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/plural"
)

func (a *app) doctorCmd() *cobra.Command {
	var fix bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Find what is wrong with the files, the work tree and git of the store, and with --fix repair what a sync repairs",
		Args:  exactArgs(0),
		RunE:  func(*cobra.Command, []string) error { return a.doctor(fix) },
	}
	cmd.Flags().BoolVar(&fix, "fix", false, "repair parent cycles, children of deleted tasks, tasks in another workspace than their parent and ids that clash, in one commit; clone a store again from origin after a force push; then tell what is left")
	return cmd
}

func (a *app) doctor(fix bool) error {
	rs, err := a.ss.Doctor(fix)
	if err != nil {
		return err
	}
	// the heading names the store when two or more are mounted, even when
	// --store narrows the report to one, as ls does
	heading := len(rs) > 1
	if !heading && !a.jsonOut {
		n, err := a.ss.MountedCount()
		if err != nil {
			return err
		}
		heading = n > 1
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
		if err := a.writeDoctor(r, fix, heading, i == 0); err != nil {
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
		if r.Rebuilt != nil {
			if err := a.writeRebuilt(r.Store, r.Rebuilt); err != nil {
				return err
			}
		}
		if len(r.Repaired) == 0 && r.Rebuilt == nil {
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

// writeRebuilt tells of a store that was cloned again from origin after a
// force push, on lines of their own: what was put back, then, when some
// changes could not be, where the old store is.
func (a *app) writeRebuilt(store string, b *ops.Rebuilt) error {
	if _, err := fmt.Fprintf(a.stdout, "Cloned %s again from origin; put back %s\n", store, plural.Count(b.Replayed, "unpushed commit")); err != nil {
		return err
	}
	if len(b.Dropped) == 0 {
		return nil
	}
	_, err := fmt.Fprintf(a.stdout, "Could not put back the changes to %s; the old store is kept at %s\n", strings.Join(b.Dropped, ", "), b.Kept)
	return err
}
