package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

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

// id colors a short ID.
func (s style) id(t string) string { return s.wrap("33", t) }

// dim marks text that is context, not a result. Without color it is put in
// parentheses.
func (s style) dim(t string) string { //nolint:unused // used by ls --tree in Task 8
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
	ID        string         `json:"id"`
	Short     string         `json:"short"`
	Key       string         `json:"key"`
	Kind      string         `json:"kind"`
	Title     string         `json:"title"`
	Parent    *string        `json:"parent"`
	Rank      string         `json:"rank"`
	Status    string         `json:"status"`
	Assignees []string       `json:"assignees"`
	Labels    []string       `json:"labels"`
	BlockedBy []string       `json:"blocked_by"`
	Props     map[string]any `json:"props"`
	Created   time.Time      `json:"created"`
	Updated   time.Time      `json:"updated"`
	DeletedAt *time.Time     `json:"deleted_at"`
	Problems  []problemJSON  `json:"problems"`
	Broken    string         `json:"broken,omitempty"`

	// Only in ls --tree --json.
	Context  bool        `json:"context,omitempty"`  // shown only to connect the tasks found
	InCycle  bool        `json:"in_cycle,omitempty"` // in a parent cycle, out of reach of the roots
	Children []*taskJSON `json:"children,omitempty"`
}

type problemJSON struct {
	Code    string `json:"code"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

func entryJSON(ix *index.Index, e *index.Entry) *taskJSON {
	j := &taskJSON{
		ID:        e.ID.String(),
		Short:     ix.ShortID(e.ID),
		Key:       e.Key,
		Kind:      e.Kind,
		Title:     e.Title,
		Rank:      string(e.Rank),
		Status:    e.Status,
		Assignees: orEmpty(e.Assignees),
		Labels:    orEmpty(e.Labels),
		BlockedBy: []string{},
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
	for _, b := range e.BlockedBy {
		j.BlockedBy = append(j.BlockedBy, b.String())
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
	for _, p := range ix.Problems(e.ID) {
		j.Problems = append(j.Problems, problemJSON{p.Code, p.Field, p.Msg})
	}
	return j
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

var taskHeader = []string{"ID", "STATUS", "KIND", "ASSIGNEES", "LABELS", "TITLE"}

// taskRow is one task in the table. The title is last, so wide characters
// in it do not shift other columns.
func taskRow(ix *index.Index, e *index.Entry) []string {
	kind, status := e.Kind, e.Status
	if e.Broken != "" {
		kind, status = "?", "?"
	}
	return []string{ix.ShortID(e.ID), status, kind, strings.Join(e.Assignees, ","), strings.Join(e.Labels, ","), e.Title}
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
		rows[i] = taskRow(ix, e)
	}
	st := a.style()
	return writeTable(a.stdout, taskHeader, rows, func(r, c int, s string) string {
		if r >= 0 && c == 0 {
			return st.id(s)
		}
		return s
	})
}

// printTask prints the task id after a write: as JSON, or as one line that
// starts with verb.
func (a *app) printTask(ix *index.Index, id model.ID, verb string) error {
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
	_, err := fmt.Fprintf(a.stdout, "%s %s %s\n", verb, a.style().id(ix.ShortID(id)), e.Title)
	return err
}
