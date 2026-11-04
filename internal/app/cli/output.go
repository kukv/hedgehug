package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"golang.org/x/text/width"

	"github.com/kukv/hedgehug/internal/app/usecase"
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

// writeTable writes rows under header, padding each column but the last
// to its widest cell. paint, when not nil, colors the padded cell of row r
// (-1 for the header) and column c; padding first keeps the escape codes
// out of the widths.
func writeTable(w io.Writer, header []string, rows [][]string, paint func(r, c int, s string) string) error {
	all := append([][]string{header}, rows...)
	widths := make([]int, len(header))
	for _, row := range all {
		for c, cell := range row {
			widths[c] = max(widths[c], displayWidth(cell))
		}
	}
	var b strings.Builder
	for r, row := range all {
		for c, cell := range row {
			if c < len(row)-1 {
				cell += strings.Repeat(" ", widths[c]-displayWidth(cell)) + "  "
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
func taskRow(v *usecase.TaskView) []string {
	id, kind, status := v.DisplayID, v.Kind, v.Status
	if id == "" {
		id = "?"
	}
	if v.Broken != "" {
		kind, status = "?", "?"
	}
	return []string{id, v.ULID, status, kind, strings.Join(v.Assignees, ","), strings.Join(v.Labels, ","), v.Title}
}

// printTasks prints vs as a table; withStore puts the name of the store of
// each task in a STORE column ahead of the others.
func (a *app) printTasks(vs []*usecase.TaskView, withStore bool) error {
	if len(vs) == 0 {
		return nil
	}
	header, lead := taskHeader, 0
	if withStore {
		header, lead = append([]string{"STORE"}, taskHeader...), 1
	}
	rows := make([][]string, len(vs))
	for i, v := range vs {
		rows[i] = taskRow(v)
		if withStore {
			rows[i] = append([]string{v.Store}, rows[i]...)
		}
	}
	st := a.style()
	return writeTable(a.stdout, header, rows, func(r, c int, s string) string {
		if r >= 0 && c >= lead && c <= lead+1 {
			return st.id(s)
		}
		return s
	})
}

// printTask prints the task after a write: as JSON, or as one line that
// starts with verb.
func (a *app) printTask(r usecase.TaskResult, verb string) error {
	w, err := r.Write()
	if err != nil {
		return err
	}
	return a.printWrite(w, verb)
}

// printWrite prints w, the answer to a write to one task: as JSON, or as one
// line that starts with verb.
func (a *app) printWrite(w usecase.TaskWrite, verb string) error {
	if a.jsonOut {
		return a.printJSON(w)
	}
	_, err := fmt.Fprintf(a.stdout, "%s %s %s\n", verb, a.style().id(w.Task.Label()), w.Task.Title)
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
