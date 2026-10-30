package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

func (a *app) showCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show a task",
		Args:  exactArgs(1),
		RunE:  func(_ *cobra.Command, args []string) error { return a.show(args[0]) },
	}
}

type commentJSON struct {
	ID      string    `json:"id"`
	Author  string    `json:"author"`
	Created time.Time `json:"created"`
	Body    string    `json:"body"`
}

type refJSON struct {
	Short string `json:"short"`
	Title string `json:"title"`
}

type showJSON struct {
	*taskJSON
	Body     string               `json:"body"`
	Watchers []string             `json:"watchers"`
	Repos    []string             `json:"repos"`
	Comments []commentJSON        `json:"comments"`
	Reverse  map[string][]refJSON `json:"reverse"`
}

// relation is one kind of task pointing at the task shown.
type relation struct {
	key   string // the name in JSON: blocks, or the property
	label string // the name in the text
	tasks []*index.Entry
}

func (a *app) show(arg string) error {
	s, ix, err := a.open()
	if err != nil {
		return err
	}
	e, err := a.resolve(ix, arg)
	if err != nil {
		return err
	}
	var t *model.Task
	if e.Broken == "" {
		if t, err = s.Task(e.ID); err != nil {
			return err
		}
	}
	var comments []*model.Comment
	if e.Broken == "" {
		if comments, err = liveComments(s, e.ID); err != nil {
			return err
		}
	}
	rels := reverse(ix, e.ID)
	if a.jsonOut {
		return a.printJSON(showOf(ix, e, t, comments, rels))
	}
	_, err = fmt.Fprint(a.stdout, a.showText(ix, e, t, comments, rels))
	return err
}

func liveComments(s *store.Store, id model.ID) ([]*model.Comment, error) {
	var out []*model.Comment
	for c, err := range s.Comments(id) {
		if err != nil {
			return nil, err
		}
		if c.DeletedAt == nil {
			out = append(out, c)
		}
	}
	return out, nil
}

// reverse finds the tasks that point at id, by relation.
func reverse(ix *index.Index, id model.ID) []relation {
	var out []relation
	if es := ix.Reverse(id, "blocked_by"); len(es) > 0 {
		out = append(out, relation{"blocks", "blocks", es})
	}
	for _, p := range ix.Schema().Props {
		if p.Type != schema.TypeTaskRef {
			continue
		}
		es := ix.Reverse(id, p.Name)
		if len(es) == 0 {
			continue
		}
		label := p.Name + " (from)"
		switch {
		case p.Inverse != "":
			label = p.Inverse
		case p.Symmetric && p.Label != "":
			label = p.Label
		case p.Symmetric:
			label = p.Name
		}
		out = append(out, relation{p.Name, label, es})
	}
	return out
}

// refName names the task id for a reader: its short ID and title.
func refName(ix *index.Index, id model.ID) string {
	if e, ok := ix.Task(id); ok {
		return ix.ShortID(id) + " " + e.Title
	}
	return id.String() + " (not found)"
}

func showOf(ix *index.Index, e *index.Entry, t *model.Task, comments []*model.Comment, rels []relation) *showJSON {
	j := &showJSON{taskJSON: entryJSON(ix, e), Watchers: []string{}, Repos: []string{}, Comments: []commentJSON{}, Reverse: map[string][]refJSON{}}
	if t != nil {
		j.Body, j.Watchers, j.Repos = t.Body, orEmpty(t.Watchers), orEmpty(t.Repos)
	}
	for _, c := range comments {
		j.Comments = append(j.Comments, commentJSON{c.ID.String(), c.Author, c.Created, c.Body})
	}
	for _, r := range rels {
		for _, x := range r.tasks {
			j.Reverse[r.key] = append(j.Reverse[r.key], refJSON{ix.ShortID(x.ID), x.Title})
		}
	}
	return j
}

func (a *app) showText(ix *index.Index, e *index.Entry, t *model.Task, comments []*model.Comment, rels []relation) string {
	st := a.style()
	var b strings.Builder
	b.WriteString(st.id(ix.ShortID(e.ID)) + "  " + e.Title + "\n")
	field := func(k string, vs ...string) {
		v := strings.Join(vs, ", ")
		if v == "" {
			v = "-"
		}
		fmt.Fprintf(&b, "  %-11s %s\n", k, v)
	}
	refs := func(ids []model.ID) []string {
		out := make([]string, len(ids))
		for i, id := range ids {
			out[i] = refName(ix, id)
		}
		return out
	}
	field("id", e.ID.String())
	field("key", e.Key)
	field("kind", e.Kind)
	field("status", e.Status)
	if e.Parent != nil {
		field("parent", refName(ix, *e.Parent))
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
	if ps := ix.Problems(e.ID); len(ps) > 0 {
		b.WriteString("\nProblems:\n")
		for _, p := range ps {
			fmt.Fprintf(&b, "  %s %s: %s\n", p.Code, p.Field, p.Msg)
		}
	}
	if len(rels) > 0 {
		b.WriteString("\nRelated:\n")
		for _, r := range rels {
			names := make([]string, len(r.tasks))
			for i, x := range r.tasks {
				names[i] = ix.ShortID(x.ID) + " " + x.Title
			}
			field(r.label, names...)
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
			fmt.Fprintf(&b, "  %s  %s\n", c.Author, c.Created.Format(time.RFC3339))
			for _, l := range strings.Split(strings.TrimRight(c.Body, "\n"), "\n") {
				b.WriteString("    " + l + "\n")
			}
		}
	}
	return b.String()
}
