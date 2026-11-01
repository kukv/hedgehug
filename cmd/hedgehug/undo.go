package main

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/ops"
)

func (a *app) undoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "undo",
		Short: "Undo your last operation, keeping what others changed after it",
		Args:  exactArgs(0),
		RunE:  func(_ *cobra.Command, _ []string) error { return a.undo() },
	}
}

type undoJSON struct {
	Undone  *string    `json:"undone"`
	Subject string     `json:"subject,omitempty"`
	Changed bool       `json:"changed"`
	Kept    []keptJSON `json:"kept"`
}

type keptJSON struct {
	Path   string `json:"path"`
	Task   string `json:"task,omitempty"`
	Field  string `json:"field"`
	Author string `json:"author"`
}

func (a *app) undo() error {
	res, err := a.ss.Undo()
	var oe *ops.Error
	if errors.As(err, &oe) && oe.Code == "undo.nothing" {
		return a.printUndo(ops.UndoResult{})
	}
	if err != nil {
		return err
	}
	return a.printUndo(res)
}

func (a *app) printUndo(res ops.UndoResult) error {
	if a.jsonOut {
		out := undoJSON{Kept: []keptJSON{}}
		if res.Found {
			h := res.Commit.Hash.String()
			out.Undone, out.Subject, out.Changed = &h, ops.UndoSubject(res.Commit), res.Changed
		}
		for _, k := range res.Kept {
			out.Kept = append(out.Kept, keptJSON{Path: k.Path, Task: k.Task, Field: k.Field, Author: k.Author})
		}
		return a.printJSON(out)
	}
	if !res.Found {
		_, err := fmt.Fprintln(a.stdout, "Nothing to undo")
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Undid %s %s\n", res.Commit.Hash.String()[:7], ops.UndoSubject(res.Commit))
	// Name the file only when the lines are about more than one.
	several := slices.ContainsFunc(res.Kept, func(k ops.UndoKept) bool { return k.Path != res.Kept[0].Path })
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
