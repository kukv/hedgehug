package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/schema"
)

func (a *app) showCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show a task",
		Args:  exactArgs(1),
		RunE:  func(_ *cobra.Command, args []string) error { return a.show(args[0]) },
	}
}

func (a *app) show(arg string) error {
	r, err := a.ss.Show(arg)
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(ops.ShowViewOf(r))
	}
	_, err = fmt.Fprint(a.stdout, a.showText(r.Index, r.Entry, r.Task, r.Comments, r.Relations, r.Back, r.History))
	return err
}

func (a *app) showText(ix *index.Index, e *index.Entry, t *model.Task, comments []*model.Comment, rels []ops.Relation, back []*index.Entry, history []gitrepo.LogEntry) string {
	st := a.style()
	var b strings.Builder
	title := e.Title
	if title == "" {
		title = "-" // a file that does not decode
	}
	b.WriteString(title + "\n")
	field := func(k string, vs ...string) {
		v := strings.Join(vs, ", ")
		if v == "" {
			v = "-"
		}
		b.WriteString("  " + k + strings.Repeat(" ", max(0, 11-displayWidth(k))) + " " + v + "\n")
	}
	refs := func(ulids []model.ULID) []string {
		out := make([]string, len(ulids))
		for i, ulid := range ulids {
			out[i] = ops.RefName(ix, ulid)
		}
		return out
	}
	field("id", e.ID)
	field("ulid", e.ULID.String())
	if !e.Workspace.IsZero() {
		ws := e.Workspace.String()
		if w, ok := ix.Workspace(e.Workspace); ok {
			ws = w.Name + " (" + w.IDPrefix + ", " + ws + ")"
		}
		field("workspace", ws)
	}
	if len(e.FormerIDs) > 0 {
		field("former_ids", e.FormerIDs...)
	}
	field("kind", e.Kind)
	field("status", e.Status)
	if e.Parent != nil {
		field("parent", ops.RefName(ix, *e.Parent))
	} else {
		field("parent")
	}
	field("assignees", e.Assignees...)
	if t != nil {
		field("watchers", t.Watchers...)
	}
	field("labels", e.Labels...)
	if t != nil {
		field("repos", t.Repos...)
	}
	field("blocked_by", refs(e.BlockedBy)...)
	sc := ix.Schema()
	for _, p := range e.Props {
		if sp, ok := sc.Prop(p.Key); ok && sp.Type == schema.TypeTaskRef {
			field(p.Key, refs(e.Targets(p.Key))...)
			continue
		}
		field(p.Key, p.Values...)
	}
	by := "-"
	if t != nil && t.CreatedBy != "" {
		by = t.CreatedBy
	}
	field("created", e.Created.Format(time.RFC3339)+" by "+by)
	field("updated", e.Updated.Format(time.RFC3339))
	if e.DeletedAt != nil {
		field("deleted", e.DeletedAt.Format(time.RFC3339))
	}
	if ps := ix.Problems(e.ULID); len(ps) > 0 {
		b.WriteString("\nProblems:\n")
		for _, p := range ps {
			fmt.Fprintf(&b, "  %s %s: %s\n", p.Code, p.Field, p.Msg)
		}
	}
	if len(rels) > 0 {
		b.WriteString("\nRelated:\n")
		for _, r := range rels {
			names := make([]string, len(r.Tasks))
			for i, x := range r.Tasks {
				names[i] = ops.RefName(ix, x.ULID)
			}
			field(r.Label, names...)
		}
	}
	if len(back) > 0 {
		b.WriteString("\nReferenced by:\n")
		for _, x := range back {
			b.WriteString("  " + ops.RefName(ix, x.ULID) + "\n")
		}
	}
	if e.Broken != "" {
		b.WriteString("\nbroken: " + e.Broken + "\n")
	}
	if t != nil && strings.TrimSpace(t.Body) != "" {
		b.WriteString("\n" + strings.TrimRight(t.Body, "\n") + "\n")
	}
	if len(comments) > 0 {
		b.WriteString("\nComments:\n")
		for _, c := range comments {
			author := c.Author
			if author == "" {
				author = "-"
			}
			fmt.Fprintf(&b, "  %s  %s  %s\n", st.id(c.ULID.String()), author, c.Created.Format(time.RFC3339))
			for _, l := range strings.Split(strings.TrimRight(c.Body, "\n"), "\n") {
				b.WriteString("    " + l + "\n")
			}
		}
	}
	if len(history) > 0 {
		b.WriteString("\nHistory:\n" + logText(ix, history))
	}
	return b.String()
}
