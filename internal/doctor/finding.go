// Package doctor finds what is wrong with a store: a schema that does not
// fit its tasks, files that do not decode, changes that are not committed.
package doctor

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
	Error   Severity = "error"
	Warning Severity = "warning"
	Info    Severity = "info"
)

// MarshalJSON writes the zero value as "error".
func (v Severity) MarshalJSON() ([]byte, error) {
	if v == "" {
		v = Error
	}
	return json.Marshal(string(v))
}

// Finding is one problem that hh schema check or hh doctor reports.
type Finding struct {
	Check    string   `json:"check"`          // the kind of check, one of checkOrder
	Where    string   `json:"where"`          // a file and the place in it, or a task: its id, ULID and title
	Msg      string   `json:"msg"`            //
	Hint     string   `json:"hint,omitempty"` // how to fix it; empty when there is no advice
	Severity Severity `json:"severity"`       // Error when empty
}

// Fails tells whether f makes hh doctor exit 1: it is an error or a warning.
func (f Finding) Fails() bool { return f.Severity != Info }

// Failing tells whether any of fs Fails.
func Failing(fs []Finding) bool { return slices.ContainsFunc(fs, Finding.Fails) }

// checkOrder is the order the findings come in.
var checkOrder = []string{"schema", "template", "prop_name", "override", "task", "file", "view", "worktree", "merge_driver", "inbox", "git", "cross_store_ref"}

// Sort orders fs by the kind of check, then by where.
func Sort(fs []Finding) {
	slices.SortStableFunc(fs, func(a, b Finding) int {
		if c := slices.Index(checkOrder, a.Check) - slices.Index(checkOrder, b.Check); c != 0 {
			return c
		}
		return strings.Compare(a.Where, b.Where)
	})
}
