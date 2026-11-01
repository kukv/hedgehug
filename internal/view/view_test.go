package view

import (
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/schema"
)

const full = `# shared by the team
layout: table # tree | kanban | table
query: "project:SH-3 -status:canceled sort:due"
table:
  columns: [id, title, status, assignee, points, due]
  group_by: parent
  width: wide
kanban:
  columns: status
  swimlane: none
color: blue
`

func TestDecodeEncodeKeepsEverything(t *testing.T) {
	t.Parallel()
	v, err := Decode("今週", []byte(full))
	if err != nil {
		t.Fatal(err)
	}
	if v.Name != "今週" || v.Layout != "table" || v.Query != "project:SH-3 -status:canceled sort:due" {
		t.Errorf("v = %+v", v)
	}
	if !slices.Equal(v.TableColumns(), []string{"id", "title", "status", "assignee", "points", "due"}) || v.GroupBy() != "parent" {
		t.Errorf("table = %+v", v.Table)
	}
	if v.KanbanColumns() != "status" || v.Swimlane() != "none" {
		t.Errorf("kanban = %+v", v.Kanban)
	}
	out, err := v.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != full {
		t.Errorf("round trip =\n%s\nwant\n%s", out, full)
	}
	v2, err := Decode("今週", out)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := v2.Encode(); err != nil || string(again) != string(out) {
		t.Errorf("second round trip = %q, %v; want %q", again, err, out)
	}
	v.Query = "assignee:@me"
	v.Table.GroupBy = "status"
	out, err = v.Encode()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"# shared by the team", "width: wide", "color: blue", "group_by: status", "assignee:@me"} {
		if !strings.Contains(string(out), s) {
			t.Errorf("after a change the file lacks %q:\n%s", s, out)
		}
	}
}

func TestDefaults(t *testing.T) {
	t.Parallel()
	v, err := Decode("x", []byte("layout: kanban\nquery: \"\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if v.Table != nil || v.Kanban != nil {
		t.Errorf("sections = %+v %+v; want none", v.Table, v.Kanban)
	}
	if !slices.Equal(v.TableColumns(), []string{"id", "title", "status", "assignee", "due"}) || v.GroupBy() != "none" ||
		v.KanbanColumns() != "status" || v.Swimlane() != "none" {
		t.Errorf("defaults = %v %s %s %s", v.TableColumns(), v.GroupBy(), v.KanbanColumns(), v.Swimlane())
	}
	out, err := v.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "layout: kanban\nquery: \"\"\n" {
		t.Errorf("encode = %q; the absent sections stay absent", out)
	}
}

func TestNewEncode(t *testing.T) {
	t.Parallel()
	v := New("x")
	v.Layout, v.Query = "table", "status:todo"
	v.Table = &Table{Columns: []string{"id", "title"}}
	out, err := v.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if want := "layout: table\nquery: status:todo\ntable:\n  columns:\n  - id\n  - title\n"; string(out) != want {
		t.Errorf("encode = %q; want %q", out, want)
	}
}

func TestDecodeErrors(t *testing.T) {
	t.Parallel()
	for _, data := range []string{"- a\n", "layout: [a]\n", "table: 3\n", "table:\n  columns: x: y\n", "a: &x 1\n"} {
		if _, err := Decode("x", []byte(data)); err == nil {
			t.Errorf("Decode(%q) = nil error", data)
		}
	}
}

func TestProblems(t *testing.T) {
	t.Parallel()
	v, err := Decode("x", []byte("layout: gantt\ntable:\n  columns: [id, id, \"\"]\n  group_by: owner\nkanban:\n  columns: kind\n  swimlane: label\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`layout "gantt" is not one of tree, kanban, table`,
		`table.columns lists "id" twice`,
		`table.columns has an empty column`,
		`table.group_by "owner" is not one of none, parent, status, assignee, project`,
		`kanban.columns "kind" is not one of status, category`,
		`kanban.swimlane "label" is not one of none, assignee, project`,
	}
	if got := v.Problems(); !slices.Equal(got, want) {
		t.Errorf("Problems =\n%q\nwant\n%q", got, want)
	}
	three, _ := Decode("x", []byte("layout: table\ntable:\n  columns: [id, id, id]\n"))
	if got := three.Problems(); !slices.Equal(got, []string{`table.columns lists "id" twice`}) {
		t.Errorf("Problems with a column three times = %q", got)
	}
	ok, _ := Decode("x", []byte("layout: tree\nquery: \"\"\n"))
	if ps := ok.Problems(); len(ps) != 0 {
		t.Errorf("Problems of a good view = %q", ps)
	}
}

func TestMissingColumns(t *testing.T) {
	t.Parallel()
	v, _ := Decode("x", []byte("layout: table\ntable:\n  columns: [id, due, gone, label, other]\n"))
	if got := v.MissingColumns(schema.Default()); !slices.Equal(got, []string{"gone", "other"}) {
		t.Errorf("MissingColumns = %q", got)
	}
	dup, _ := Decode("x", []byte("layout: table\ntable:\n  columns: [gone, gone, other, gone]\n"))
	if got := dup.MissingColumns(schema.Default()); !slices.Equal(got, []string{"gone", "other"}) {
		t.Errorf("MissingColumns with repeats = %q", got)
	}
}
