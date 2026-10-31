package main

import (
	"fmt"
)

// restoreAt puts the task back as it was at the point at names; see
// ops.Session.RestoreAt.
func (a *app) restoreAt(arg, at string) error {
	res, err := a.ss.RestoreAt(arg, at)
	if err != nil {
		return err
	}
	if err := a.printTask(res.Index, res.ULID, verb(res.Committed, "Restored")); err != nil || res.Note == "" {
		return err
	}
	// JSON on stdout stays one document.
	out := a.stdout
	if a.jsonOut {
		out = a.stderr
	}
	_, err = fmt.Fprintln(out, "  "+res.Note)
	return err
}
