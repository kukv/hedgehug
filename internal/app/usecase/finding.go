package usecase

import (
	"encoding/json"
	"slices"
	"strings"
)

// Severity is how much a finding weighs. The zero value is Error, which every
// check of a single store reports; it encodes as "error".
type Severity string

// The severities. Error and Warning make hh doctor exit 1; Info alone does not.
const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// MarshalJSON writes the zero value as "error".
func (v Severity) MarshalJSON() ([]byte, error) {
	if v == "" {
		v = SeverityError
	}
	return json.Marshal(string(v))
}

// Finding is one problem that hh schema check or hh doctor reports.
type Finding struct {
	Check    string   `json:"check"`          // the kind of check, one of checkOrder
	Where    string   `json:"where"`          // a file and the place in it, or a task: its id, ULID and title
	Task     string   `json:"task,omitempty"` // the ULID of the task, for a finding about one task of the store
	Msg      string   `json:"msg"`            //
	Hint     string   `json:"hint,omitempty"` // how to fix it; empty when there is no advice
	Severity Severity `json:"severity"`       // Error when empty
}

// Fails tells whether f makes hh doctor exit 1: it is an error or a warning.
func (f Finding) Fails() bool { return f.Severity != SeverityInfo }

// FindingsFail tells whether any of fs Fails.
func FindingsFail(fs []Finding) bool { return slices.ContainsFunc(fs, Finding.Fails) }

// checkOrder is the order the findings come in.
var checkOrder = []string{"schema", "template", "prop_name", "override", "automation", "task", "file", "view", "worktree", "merge_driver", "inbox", "git", "cross_store_ref", "duplicate_across_stores", "forward_target", "unexpected_force_push"}

// SortFindings orders fs by the kind of check, then by where.
func SortFindings(fs []Finding) {
	slices.SortStableFunc(fs, func(a, b Finding) int {
		if c := slices.Index(checkOrder, a.Check) - slices.Index(checkOrder, b.Check); c != 0 {
			return c
		}
		return strings.Compare(a.Where, b.Where)
	})
}
