package cli

import (
	"fmt"

	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
)

// restoreAt puts the task back as it was at the point at names; see
// usecase.Session.RestoreAt.
func (a *app) restoreAt(arg, at string) error {
	res, err := tasks.RestoreAt(a.ss, arg, at)
	if err != nil {
		return err
	}
	w, err := res.Write()
	if err != nil {
		return err
	}
	if err := a.printWrite(w, verb(res.Committed, "Restored")); err != nil || a.jsonOut || w.Note == "" {
		// In JSON the note is in the document.
		return err
	}
	_, err = fmt.Fprintln(a.stdout, "  "+w.Note)
	return err
}
