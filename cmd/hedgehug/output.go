package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/width"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// style colors text when the output is a terminal.
type style struct{ on bool }

func (a *app) style() style {
	return style{on: !a.noColor && a.getenv("NO_COLOR") == "" && a.isTerminal(a.stdout)}
}

func (s style) wrap(code, t string) string {
	if !s.on {
		return t
	}
	return "\x1b[" + code + "m" + t + "\x1b[0m"
}

// id colors the id and ULID of a task, or the ULID of a comment.
func (s style) id(t string) string { return s.wrap("33", t) }

// dim marks text that is context, not a result. Without color it is put in
// parentheses.
func (s style) dim(t string) string {
	if !s.on {
		return "(" + t + ")"
	}
	return s.wrap("2", t)
}

// printJSON writes v as indented JSON.
func (a *app) printJSON(v any) error {
	enc := json.NewEncoder(a.stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// taskJSON is a task as --json prints it.
type taskJSON struct {
	ULID      string            `json:"ulid"`
	ID        string            `json:"id"`
	Workspace *workspaceRefJSON `json:"workspace"`
	Kind      string            `json:"kind"`
	Title     string            `json:"title"`
	Parent    *string           `json:"parent"`
	Rank      string            `json:"rank"`
	Status    string            `json:"status"`
	Assignees []string          `json:"assignees"`
	Labels    []string          `json:"labels"`
	BlockedBy []refJSON         `json:"blocked_by"`
	Props     map[string]any    `json:"props"`
	Created   time.Time         `json:"created"`
	Updated   time.Time         `json:"updated"`
	DeletedAt *time.Time        `json:"deleted_at"`
	Problems  []problemJSON     `json:"problems"`
	Broken    string            `json:"broken,omitempty"`

	// Only in ls --tree --json.
	Context  bool        `json:"context,omitempty"`  // shown only to connect the tasks found
	InCycle  bool        `json:"in_cycle,omitempty"` // in a parent cycle, out of reach of the roots
	Children []*taskJSON `json:"children,omitempty"`
}

// workspaceRefJSON names the workspace of a task.
type workspaceRefJSON struct {
	ULID     string `json:"ulid"`
	Name     string `json:"name"`
	IDPrefix string `json:"id_prefix"`
}

// refJSON names a task that another task points at.
type refJSON struct {
	ULID  string `json:"ulid"`
	ID    string `json:"id"`
	Title string `json:"title"`
}

type problemJSON struct {
	Code    string `json:"code"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

func entryJSON(ix *index.Index, e *index.Entry) *taskJSON {
	j := &taskJSON{
		ULID:      e.ULID.String(),
		ID:        e.ID,
		Kind:      e.Kind,
		Title:     e.Title,
		Rank:      string(e.Rank),
		Status:    e.Status,
		Assignees: orEmpty(e.Assignees),
		Labels:    orEmpty(e.Labels),
		BlockedBy: []refJSON{},
		Props:     map[string]any{},
		Created:   e.Created,
		Updated:   e.Updated,
		DeletedAt: e.DeletedAt,
		Problems:  []problemJSON{},
		Broken:    e.Broken,
	}
	if e.Parent != nil {
		p := e.Parent.String()
		j.Parent = &p
	}
	if !e.Workspace.IsZero() {
		j.Workspace = &workspaceRefJSON{ULID: e.Workspace.String()}
		if w, ok := ix.Workspace(e.Workspace); ok {
			j.Workspace.Name, j.Workspace.IDPrefix = w.Name, w.IDPrefix
		}
	}
	for _, b := range e.BlockedBy {
		j.BlockedBy = append(j.BlockedBy, refOf(ix, b))
	}
	sc := ix.Schema()
	for _, p := range e.Props {
		sp, ok := sc.Prop(p.Key)
		multi := (ok && (sp.Multi || sp.Type == schema.TypeMultiEnum)) || (!ok && len(p.Values) != 1)
		switch {
		case multi:
			j.Props[p.Key] = orEmpty(p.Values)
		case len(p.Values) > 0:
			j.Props[p.Key] = p.Values[0]
		default:
			j.Props[p.Key] = nil
		}
	}
	for _, p := range ix.Problems(e.ULID) {
		j.Problems = append(j.Problems, problemJSON{p.Code, p.Field, p.Msg})
	}
	return j
}

// refOf names the task id, with an empty id and title when the index does
// not have it.
func refOf(ix *index.Index, id model.ULID) refJSON {
	r := refJSON{ULID: id.String()}
	if e, ok := ix.Task(id); ok {
		r.ID, r.Title = e.ID, e.Title
	}
	return r
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// writeTable writes rows under header, padding each column but the last
// to its widest cell. paint, when not nil, colors the padded cell of row r
// (-1 for the header) and column c; padding first keeps the escape codes
// out of the widths.
func writeTable(w io.Writer, header []string, rows [][]string, paint func(r, c int, s string) string) error {
	all := append([][]string{header}, rows...)
	widths := make([]int, len(header))
	for _, row := range all {
		for c, cell := range row {
			widths[c] = max(widths[c], utf8.RuneCountInString(cell))
		}
	}
	var b strings.Builder
	for r, row := range all {
		for c, cell := range row {
			if c < len(row)-1 {
				cell += strings.Repeat(" ", widths[c]-utf8.RuneCountInString(cell)) + "  "
			}
			if paint != nil {
				cell = paint(r-1, c, cell)
			}
			b.WriteString(cell)
		}
		b.WriteString("\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

var taskHeader = []string{"ID", "ULID", "STATUS", "KIND", "ASSIGNEES", "LABELS", "TITLE"}

// taskRow is one task in the table. The title is last, so wide characters
// in it do not shift other columns.
func taskRow(e *index.Entry) []string {
	id, kind, status := e.ID, e.Kind, e.Status
	if id == "" {
		id = "?"
	}
	if e.Broken != "" {
		kind, status = "?", "?"
	}
	return []string{id, e.ULID.String(), status, kind, strings.Join(e.Assignees, ","), strings.Join(e.Labels, ","), e.Title}
}

// taskLabel names a task for a reader: its id and full ULID, or the ULID
// alone when the task has no id (a file that does not decode).
func taskLabel(e *index.Entry) string {
	if e.ID == "" {
		return e.ULID.String()
	}
	return e.ID + " (" + e.ULID.String() + ")"
}

// taskFileName names the temporary file that edits task e: its id, or its
// ULID when it has no id.
func taskFileName(e *index.Entry) string {
	if e.ID == "" {
		return e.ULID.String()
	}
	return e.ID
}

// refName names the task id for a reader: its id, ULID and title.
func refName(ix *index.Index, id model.ULID) string {
	if e, ok := ix.Task(id); ok {
		return taskLabel(e) + " " + e.Title
	}
	return id.String() + " (not found)"
}

// printTasks prints es as JSON or as a table.
func (a *app) printTasks(ix *index.Index, es []*index.Entry) error {
	if a.jsonOut {
		out := make([]*taskJSON, len(es))
		for i, e := range es {
			out[i] = entryJSON(ix, e)
		}
		return a.printJSON(out)
	}
	if len(es) == 0 {
		return nil
	}
	rows := make([][]string, len(es))
	for i, e := range es {
		rows[i] = taskRow(e)
	}
	st := a.style()
	return writeTable(a.stdout, taskHeader, rows, func(r, c int, s string) string {
		if r >= 0 && c <= 1 {
			return st.id(s)
		}
		return s
	})
}

// printTask prints the task id after a write: as JSON, or as one line that
// starts with verb.
func (a *app) printTask(ix *index.Index, id model.ULID, verb string) error {
	if err := ix.Refresh(); err != nil {
		return err
	}
	e, ok := ix.Task(id)
	if !ok {
		return fmt.Errorf("task %s is not in the index after the write", id)
	}
	if a.jsonOut {
		return a.printJSON(entryJSON(ix, e))
	}
	_, err := fmt.Fprintf(a.stdout, "%s %s %s\n", verb, a.style().id(taskLabel(e)), e.Title)
	return err
}

// displayWidth is the number of terminal columns s takes: two for East
// Asian wide and fullwidth characters, one for the others.
func displayWidth(s string) int {
	n := 0
	for _, r := range s {
		switch width.LookupRune(r).Kind() {
		case width.EastAsianWide, width.EastAsianFullwidth:
			n += 2
		default:
			n++
		}
	}
	return n
}
