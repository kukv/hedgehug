// Package doctor finds what is wrong with a store: a schema that does not
// fit its tasks, files that do not decode, changes that are not committed.
package doctor

import (
	"slices"
	"strings"
)

// Finding is one problem that hh schema check or hh doctor reports.
type Finding struct {
	Check string `json:"check"`          // the kind of check, one of checkOrder
	Where string `json:"where"`          // a file and the place in it, or a task: its key, ULID and title
	Msg   string `json:"msg"`            //
	Hint  string `json:"hint,omitempty"` // how to fix it; empty when there is no advice
}

// checkOrder is the order the findings come in.
var checkOrder = []string{"schema", "template", "prop_name", "override", "task", "file", "worktree", "git"}

// Sort orders fs by the kind of check, then by where.
func Sort(fs []Finding) {
	slices.SortStableFunc(fs, func(a, b Finding) int {
		if c := slices.Index(checkOrder, a.Check) - slices.Index(checkOrder, b.Check); c != 0 {
			return c
		}
		return strings.Compare(a.Where, b.Where)
	})
}
