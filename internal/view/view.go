package view

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// The values each setting takes.
var (
	Layouts             = []string{"tree", "kanban", "table"}
	GroupBys            = []string{"none", "parent", "status", "assignee", "project"}
	KanbanColumnsValues = []string{"status", "category"}
	Swimlanes           = []string{"none", "assignee", "project"}
)

// The defaults of the settings a view leaves out.
var (
	DefaultColumns = []string{"id", "title", "status", "assignee", "due"}
)

const (
	DefaultGroupBy       = "none"
	DefaultKanbanColumns = "status"
	DefaultSwimlane      = "none"
)

// Table is the table: section of a view.
type Table struct {
	Columns []string `yaml:"columns,omitempty"`
	GroupBy string   `yaml:"group_by,omitempty"`
}

// Kanban is the kanban: section of a view.
type Kanban struct {
	Columns  string `yaml:"columns,omitempty"`
	Swimlane string `yaml:"swimlane,omitempty"`
}

// View is one saved view. A section that the file does not have is nil;
// its settings take the defaults. Unknown keys, at the top and in the
// sections, and comments are kept when the view is written back.
type View struct {
	Name   string // from the file name
	Layout string
	Query  string
	Table  *Table
	Kanban *Kanban
	Extra  model.Fields

	doc *model.Doc
}

var keys = []string{"layout", "query", "table", "kanban"}

// New is an empty view called name.
func New(name string) *View { return &View{Name: name} }

// Decode reads the file of the view called name. It fails only on what
// cannot be read: YAML that is not a mapping, or a value of the wrong shape.
// Values out of their range are for Problems.
func Decode(name string, data []byte) (*View, error) {
	d, err := model.ParseDoc(data, 0)
	if err != nil {
		return nil, err
	}
	v := &View{Name: name, doc: d}
	for _, f := range []struct {
		key string
		out any
	}{{"layout", &v.Layout}, {"query", &v.Query}, {"table", &v.Table}, {"kanban", &v.Kanban}} {
		if err := d.Decode(f.key, f.out); err != nil {
			return nil, err
		}
	}
	v.Extra = d.Extra(keys)
	return v, nil
}

// Encode writes the view, keeping what the file it was read from had.
func (v *View) Encode() ([]byte, error) {
	if v.doc == nil {
		v.doc = model.NewDoc()
	}
	e := v.doc.Encoder()
	e.Put("layout", v.Layout, false)
	e.Put("query", v.Query, false)
	if v.Table != nil {
		e.Put("table", v.Table, false)
	}
	if v.Kanban != nil {
		e.Put("kanban", v.Kanban, false)
	}
	return e.Bytes(v.Extra)
}

// TableColumns are the columns of the table, or the default ones.
func (v *View) TableColumns() []string {
	if v.Table == nil || len(v.Table.Columns) == 0 {
		return slices.Clone(DefaultColumns)
	}
	return slices.Clone(v.Table.Columns)
}

// GroupBy is what the table groups by, or none.
func (v *View) GroupBy() string {
	if v.Table == nil || v.Table.GroupBy == "" {
		return DefaultGroupBy
	}
	return v.Table.GroupBy
}

// KanbanColumns is what the columns of the kanban are, or status.
func (v *View) KanbanColumns() string {
	if v.Kanban == nil || v.Kanban.Columns == "" {
		return DefaultKanbanColumns
	}
	return v.Kanban.Columns
}

// Swimlane is what the kanban splits its rows by, or none.
func (v *View) Swimlane() string {
	if v.Kanban == nil || v.Kanban.Swimlane == "" {
		return DefaultSwimlane
	}
	return v.Kanban.Swimlane
}

// Problems lists the values of the view that are out of their range, in the
// order of the file's keys. The columns are checked against the schema by
// MissingColumns.
func (v *View) Problems() []string {
	var out []string
	oneOf := func(what, val string, vals []string) {
		if !slices.Contains(vals, val) {
			out = append(out, fmt.Sprintf("%s %q is not one of %s", what, val, strings.Join(vals, ", ")))
		}
	}
	oneOf("layout", v.Layout, Layouts)
	if v.Table != nil {
		seen := map[string]int{}
		empty := false
		for _, c := range v.Table.Columns {
			switch {
			case c == "":
				empty = true
			case seen[c] == 1:
				out = append(out, fmt.Sprintf("table.columns lists %q twice", c))
			}
			seen[c]++
		}
		if empty {
			out = append(out, "table.columns has an empty column")
		}
		if v.Table.GroupBy != "" {
			oneOf("table.group_by", v.Table.GroupBy, GroupBys)
		}
	}
	if v.Kanban != nil {
		if v.Kanban.Columns != "" {
			oneOf("kanban.columns", v.Kanban.Columns, KanbanColumnsValues)
		}
		if v.Kanban.Swimlane != "" {
			oneOf("kanban.swimlane", v.Kanban.Swimlane, Swimlanes)
		}
	}
	return out
}

// MissingColumns lists the columns of the table that are neither a builtin
// field nor a property of sc, in their order. The GUI skips them.
func (v *View) MissingColumns(sc *schema.Schema) []string {
	var out []string
	if v.Table == nil {
		return nil
	}
	for _, c := range v.Table.Columns {
		if c == "" || IsBuiltin(c) || slices.Contains(out, c) {
			continue
		}
		if _, ok := sc.Prop(c); !ok {
			out = append(out, c)
		}
	}
	return out
}
