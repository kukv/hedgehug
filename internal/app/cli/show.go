package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/browse"
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
	v, err := browse.Show(a.ss, arg)
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(v)
	}
	_, err = fmt.Fprint(a.stdout, a.showText(v))
	return err
}

func (a *app) showText(v *usecase.ShowView) string {
	st := a.style()
	var b strings.Builder
	field := func(k string, vs ...string) {
		v := strings.Join(vs, ", ")
		if v == "" {
			v = "-"
		}
		b.WriteString("  " + k + strings.Repeat(" ", max(0, 11-displayWidth(k))) + " " + v + "\n")
	}
	names := func(refs []usecase.Ref) []string {
		out := make([]string, len(refs))
		for i, r := range refs {
			out[i] = r.Name()
		}
		return out
	}
	field("title", v.Title) // "-" for a file that does not decode
	field("id", v.DisplayID)
	field("ulid", v.ULID)
	field("store", v.Store)
	if w := v.Workspace; w != nil {
		ws := w.ULID
		if w.Known {
			ws = w.Name + " (" + w.IDPrefix + ", " + ws + ")"
		}
		field("workspace", ws)
	}
	if len(v.FormerDisplayIDs) > 0 {
		field("former_ids", v.FormerDisplayIDs...)
	}
	field("kind", v.Kind)
	field("status", v.Status)
	if c := usecase.ChecklistText(v.Checklist); c != "" {
		field("checklist", c)
	}
	if p := usecase.ProgressFractionText(v.Progress); p != "" {
		field("progress", p)
	}
	if v.ParentRef != nil {
		field("parent", v.ParentRef.Name())
	} else {
		field("parent")
	}
	field("assignees", v.Assignees...)
	if v.Broken == "" {
		field("watchers", v.Watchers...)
	}
	field("labels", v.Labels...)
	if v.Broken == "" {
		field("repos", v.Repos...)
	}
	field("blocked_by", names(v.BlockedBy)...)
	for _, p := range v.PropList {
		if p.IsRef {
			field(p.Key, names(p.Refs)...)
			continue
		}
		field(p.Key, p.Values...)
	}
	by := "-"
	if v.CreatedBy != "" {
		by = v.CreatedBy
	}
	field("created", v.Created.Format(time.RFC3339)+" by "+by)
	field("updated", v.Updated.Format(time.RFC3339))
	if v.DeletedAt != nil {
		field("deleted", v.DeletedAt.Format(time.RFC3339))
	}
	if len(v.Problems) > 0 {
		b.WriteString("\nProblems:\n")
		for _, p := range v.Problems {
			fmt.Fprintf(&b, "  %s %s: %s\n", p.Code, p.Field, p.Message)
		}
	}
	if len(v.Relations) > 0 {
		b.WriteString("\nRelated:\n")
		for _, r := range v.Relations {
			field(r.Label, names(r.Tasks)...)
		}
	}
	if len(v.ReferencedBy) > 0 {
		b.WriteString("\nReferenced by:\n")
		for _, x := range v.ReferencedBy {
			b.WriteString("  " + x.Name() + "\n")
		}
	}
	if len(v.Links) > 0 {
		b.WriteString("\nLinks:\n")
		for _, l := range v.Links {
			b.WriteString("  " + linkLine(l) + "\n")
		}
	}
	if v.Broken != "" {
		b.WriteString("\nbroken: " + v.Broken + "\n")
	}
	if v.Broken == "" && strings.TrimSpace(v.Body) != "" {
		b.WriteString("\n" + strings.TrimRight(v.Body, "\n") + "\n")
	}
	if len(v.Comments) > 0 {
		b.WriteString("\nComments:\n")
		for _, c := range v.Comments {
			author := c.Author
			if author == "" {
				author = "-"
			}
			fmt.Fprintf(&b, "  %s  %s  %s\n", st.id(c.ULID), author, c.Created.Format(time.RFC3339))
			for _, l := range strings.Split(strings.TrimRight(c.Body, "\n"), "\n") {
				b.WriteString("    " + l + "\n")
			}
		}
	}
	if len(v.History) > 0 {
		b.WriteString("\nHistory:\n" + logText(v.History))
	}
	return b.String()
}

// linkLine is one link of hedgehug show.
func linkLine(l usecase.LinkView) string {
	kind := "issue"
	if l.Type == "github_pr" {
		kind = "pull request"
	}
	parts := []string{kind, l.Label}
	switch {
	case l.State == "":
		parts = append(parts, "not read yet")
	case l.Type == "github_pr":
		parts = append(parts, l.State, "review: "+l.Review)
		if l.Checks != "" {
			parts = append(parts, "checks: "+l.Checks)
		}
	default:
		parts = append(parts, l.State, "sync: "+l.Sync)
	}
	return strings.Join(parts, "  ")
}
