package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/gitrepo"
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
	ULID    string    `json:"ulid"`
	Author  string    `json:"author"`
	Created time.Time `json:"created"`
	Body    string    `json:"body"`
}

type showJSON struct {
	*taskJSON
	Body     string               `json:"body"`
	Watchers []string             `json:"watchers"`
	Repos    []string             `json:"repos"`
	Comments []commentJSON        `json:"comments"`
	Reverse  map[string][]refJSON `json:"reverse"`
	History  []logJSON            `json:"history"`
}

// showHistory is how many commits show lists under History.
const showHistory = 5

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
		if t, err = s.Task(e.ULID); err != nil {
			return err
		}
	}
	var comments []*model.Comment
	if e.Broken == "" {
		if comments, err = liveComments(s, e.ULID); err != nil {
			return err
		}
	}
	rels := reverse(ix, e.ULID)
	history, err := a.taskLog(s, e.ULID, showHistory)
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(showOf(ix, e, t, comments, rels, history))
	}
	_, err = fmt.Fprint(a.stdout, a.showText(ix, e, t, comments, rels, history))
	return err
}

func liveComments(s *store.Store, id model.ULID) ([]*model.Comment, error) {
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
func reverse(ix *index.Index, id model.ULID) []relation {
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

func showOf(ix *index.Index, e *index.Entry, t *model.Task, comments []*model.Comment, rels []relation, history []gitrepo.LogEntry) *showJSON {
	j := &showJSON{taskJSON: entryJSON(ix, e), Watchers: []string{}, Repos: []string{}, Comments: []commentJSON{}, Reverse: map[string][]refJSON{}, History: logJSONs(ix, history)}
	if t != nil {
		j.Body, j.Watchers, j.Repos = t.Body, orEmpty(t.Watchers), orEmpty(t.Repos)
	}
	for _, c := range comments {
		j.Comments = append(j.Comments, commentJSON{c.ULID.String(), c.Author, c.Created, c.Body})
	}
	for _, r := range rels {
		for _, x := range r.tasks {
			j.Reverse[r.key] = append(j.Reverse[r.key], refJSON{x.ULID.String(), x.ID, x.Title})
		}
	}
	return j
}

func (a *app) showText(ix *index.Index, e *index.Entry, t *model.Task, comments []*model.Comment, rels []relation, history []gitrepo.LogEntry) string {
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
	refs := func(ids []model.ULID) []string {
		out := make([]string, len(ids))
		for i, id := range ids {
			out[i] = refName(ix, id)
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
	if ps := ix.Problems(e.ULID); len(ps) > 0 {
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
				names[i] = refName(ix, x.ULID)
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
