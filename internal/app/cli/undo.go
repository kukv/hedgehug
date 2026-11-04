package cli

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
)

func (a *app) undoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "undo",
		Short: "Undo your last operation, keeping what others changed after it",
		Args:  exactArgs(0),
		RunE:  func(_ *cobra.Command, _ []string) error { return a.undo() },
	}
}

func (a *app) undo() error {
	res, err := tasks.Undo(a.ss)
	var oe *usecase.Error
	if errors.As(err, &oe) && oe.Code == "undo.nothing" {
		return a.printUndo(tasks.UndoResult{})
	}
	if err != nil {
		return err
	}
	return a.printUndo(res)
}

func (a *app) printUndo(res tasks.UndoResult) error {
	if a.jsonOut {
		return a.printJSON(res.Write())
	}
	if !res.Found {
		_, err := fmt.Fprintln(a.stdout, "Nothing to undo")
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Undid %s %s\n", res.Undone.Hash[:7], tasks.UndoSubject(res.Undone.Subject))
	// Name the file only when the lines are about more than one.
	several := slices.ContainsFunc(res.Kept, func(k tasks.UndoKept) bool { return k.Path != res.Kept[0].Path })
	for _, k := range res.Kept {
		fmt.Fprintf(&b, "  kept %s (changed later by %s)", k.Field, k.Author)
		if several {
			b.WriteString(" in " + cmp.Or(k.Label, k.Path))
		}
		b.WriteString("\n")
	}
	if !res.Changed {
		b.WriteString("  nothing to change: every field was changed later by others\n")
	}
	_, err := fmt.Fprint(a.stdout, b.String())
	return err
}
