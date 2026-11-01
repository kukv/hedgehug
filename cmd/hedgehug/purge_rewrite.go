package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/ops"
)

// purgeRewriteHistory removes the task and the tasks under it from the git
// history of its store (spec "履歴の書き換え"). The question is the store
// name, on a terminal only: --yes does not go through (decision 17).
func (a *app) purgeRewriteHistory(arg string) error {
	in := ops.RewriteInput{Arg: arg}
	if !a.isTerminal(a.stdin) {
		// the refusal the operation gives; nothing is synced or changed
		_, err := a.ss.RewriteHistory(in)
		return err
	}
	plan, err := a.ss.PrepareRewrite(in)
	if err != nil {
		return err
	}
	var b strings.Builder
	if plan.Resumed {
		fmt.Fprintf(&b, "An earlier run did not finish: the history of %s is rewritten here, and the push is left.\n", plan.Store)
	}
	for _, t := range plan.Tasks {
		b.WriteString("  " + rewriteTargetLabel(t) + "\n")
	}
	fmt.Fprintf(&b, "This rewrites the history of every remote of %s and force pushes it. The clones of other members are rebuilt at their next sync.\n", plan.Store)
	b.WriteString("Type the store name to go ahead: ")
	if _, err := fmt.Fprint(a.stderr, b.String()); err != nil {
		return err
	}
	line, err := a.readLine()
	if err != nil && line == "" {
		return errors.New("Canceled")
	}
	in.Typed, in.Confirmed = strings.TrimSpace(line), plan.ULIDs()
	if in.Typed == "" {
		return errors.New("Canceled")
	}
	res, err := a.ss.RewriteHistory(in)
	if err != nil {
		return err
	}
	return a.printRewritten(res)
}

// rewriteTargetLabel is a task of the plan as the list shows it: its id (the
// ULID when it has none that is known) and title.
func rewriteTargetLabel(t ops.RewriteTarget) string {
	label := t.ID
	if label == "" {
		label = t.ULID.String()
	}
	if t.Title != "" {
		label += " " + t.Title
	}
	return label
}

// printRewritten prints what the rewrite did: a line, or JSON.
func (a *app) printRewritten(res ops.RewriteResult) error {
	for _, w := range res.Warnings {
		if _, err := fmt.Fprintln(a.stderr, "warning: "+w); err != nil {
			return err
		}
	}
	if a.jsonOut {
		type task struct {
			ULID string `json:"ulid"`
			ID   string `json:"id,omitempty"`
		}
		out := struct {
			Store     string   `json:"store"`
			Tasks     []task   `json:"tasks"`
			Rewritten int      `json:"rewritten"`
			Pushed    []string `json:"pushed"`
		}{res.Plan.Store, []task{}, res.Rewritten, append([]string{}, res.Pushed...)}
		for _, t := range res.Plan.Tasks {
			out.Tasks = append(out.Tasks, task{t.ULID.String(), t.ID})
		}
		return a.printJSON(out)
	}
	_, err := fmt.Fprintf(a.stdout, "Removed %d %s from the history (%d %s rewritten)\n",
		len(res.Plan.Tasks), plural(len(res.Plan.Tasks), "task", "tasks"), res.Rewritten, plural(res.Rewritten, "commit", "commits"))
	return err
}
