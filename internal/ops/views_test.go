package ops

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/view"
)

func ptr[T any](v T) *T { return &v }

func (ts *testSession) viewFile(name string) string {
	ts.t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		ts.t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(s.Dir(), "views", name+".yaml"))
	if err != nil {
		ts.t.Fatal(err)
	}
	return string(b)
}

func (ts *testSession) headSubject() string {
	ts.t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		ts.t.Fatal(err)
	}
	es, err := s.Repo().Log(nil, 1)
	if err != nil || len(es) == 0 {
		ts.t.Fatalf("log: %v", err)
	}
	return es[0].Subject
}

func (ts *testSession) headTrailer(key string) (string, bool) {
	ts.t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		ts.t.Fatal(err)
	}
	es, err := s.Repo().Log(nil, 1)
	if err != nil || len(es) == 0 {
		ts.t.Fatalf("log: %v", err)
	}
	return es[0].Trailer(key)
}

func TestViewSaveCreateAndOverwrite(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	r, err := ts.ViewSave("今週", ViewChange{})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Committed || r.Commit == "" || r.View.Layout != "tree" || r.View.Query != "" {
		t.Errorf("create = %+v %+v", r, r.View)
	}
	if got := ts.headSubject(); got != "hedgehug: view save 今週" {
		t.Errorf("subject = %q", got)
	}
	if got, _ := ts.headTrailer("Hedgehug-Op"); got != "view" {
		t.Errorf("Hedgehug-Op = %q", got)
	}
	if got := ts.viewFile("今週"); got != "layout: tree\nquery: \"\"\n" {
		t.Errorf("file = %q", got)
	}
	r, err = ts.ViewSave("今週", ViewChange{Layout: ptr("table"), Columns: ptr([]string{"id", "title", "due"})})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Committed || r.View.Layout != "table" || !slices.Equal(r.View.TableColumns(), []string{"id", "title", "due"}) {
		t.Errorf("overwrite = %+v %+v", r, r.View)
	}
	// Only the flags given change; a save that changes nothing commits nothing.
	r, err = ts.ViewSave("今週", ViewChange{Query: ptr("status:todo")})
	if err != nil || r.View.Layout != "table" || r.View.Query != "status:todo" {
		t.Fatalf("query only = %+v, %v", r, err)
	}
	r, err = ts.ViewSave("今週", ViewChange{Query: ptr("status:todo")})
	if err != nil || r.Committed || r.Commit != "" {
		t.Errorf("same again = %+v, %v", r, err)
	}
}

func TestViewSaveKeepsUnknownKeys(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	if _, err := ts.ViewSave("a", ViewChange{Layout: ptr("table")}); err != nil {
		t.Fatal(err)
	}
	ts.writeAndCommit("views/a.yaml", "# note\nlayout: table\nquery: \"\"\ncolor: red\n")
	if _, err := ts.ViewSave("a", ViewChange{Query: ptr("status:todo")}); err != nil {
		t.Fatal(err)
	}
	got := ts.viewFile("a")
	for _, w := range []string{"# note", "color: red", "status:todo"} {
		if !strings.Contains(got, w) {
			t.Errorf("file lacks %q:\n%s", w, got)
		}
	}
}

func TestViewSaveRefuses(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	if _, err := ts.ViewSave("Release", ViewChange{}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		ch   ViewChange
		kind Kind
		code string
	}{
		{"a/b", ViewChange{}, KindUsage, "view.bad_name"},
		{"CON", ViewChange{}, KindUsage, "view.bad_name"},
		{"release", ViewChange{}, KindConflict, "view.name_taken"},
		{"x", ViewChange{Layout: ptr("gantt")}, KindUsage, "view.bad_value"},
		{"x", ViewChange{Layout: ptr("table"), GroupBy: ptr("owner")}, KindUsage, "view.bad_value"},
		{"x", ViewChange{KanbanColumns: ptr("kind")}, KindUsage, "view.bad_value"},
		{"x", ViewChange{Swimlane: ptr("label")}, KindUsage, "view.bad_value"},
		{"x", ViewChange{Columns: ptr([]string{"id", "id"})}, KindUsage, "view.bad_value"},
		{"x", ViewChange{Columns: ptr([]string{"id", "nope"})}, KindInvalid, "view.bad_column"},
		{"x", ViewChange{Query: ptr("status:todo sort:due")}, KindUsage, "view.tree_order"},
		{"x", ViewChange{Layout: ptr("tree"), Query: ptr("group:status")}, KindUsage, "view.tree_order"},
		{"x", ViewChange{Query: ptr(`"abc`)}, KindUsage, "query.invalid"},
	} {
		_, err := ts.ViewSave(c.name, c.ch)
		wantErr(t, err, c.kind, c.code)
	}
	// A table may sort; switching it to tree then is refused.
	if _, err := ts.ViewSave("t", ViewChange{Layout: ptr("table"), Query: ptr("sort:due")}); err != nil {
		t.Fatal(err)
	}
	_, err := ts.ViewSave("t", ViewChange{Layout: ptr("tree")})
	wantErr(t, err, KindUsage, "view.tree_order")
	if err.Error() != "a tree view orders tasks by rank; its query takes neither group: nor sort:" {
		t.Errorf("message = %q", err)
	}
}

func TestViewListGetRemoveRename(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	if vs, err := ts.ViewList(); err != nil || len(vs) != 0 {
		t.Fatalf("empty list = %v, %v", vs, err)
	}
	for _, n := range []string{"b", "a"} {
		if _, err := ts.ViewSave(n, ViewChange{Layout: ptr("kanban"), Query: ptr("label:" + n)}); err != nil {
			t.Fatal(err)
		}
	}
	vs, err := ts.ViewList()
	if err != nil {
		t.Fatal(err)
	}
	want := []SavedView{
		{Name: "a", Layout: "kanban", Query: "label:a", Table: SavedTable{Columns: view.DefaultColumns, GroupBy: "none"}, Kanban: SavedKanban{Columns: "status", Swimlane: "none"}},
		{Name: "b", Layout: "kanban", Query: "label:b", Table: SavedTable{Columns: view.DefaultColumns, GroupBy: "none"}, Kanban: SavedKanban{Columns: "status", Swimlane: "none"}},
	}
	if len(vs) != 2 || vs[0].Name != want[0].Name || vs[1].Query != want[1].Query || !slices.Equal(vs[0].Table.Columns, want[0].Table.Columns) || vs[0].Kanban != want[0].Kanban {
		t.Errorf("list = %+v", vs)
	}
	v, err := ts.ViewGet("a")
	if err != nil || v.Query != "label:a" {
		t.Errorf("get = %+v, %v", v, err)
	}
	_, err = ts.ViewGet("A")
	wantErr(t, err, KindNotFound, "view.not_found")

	r, err := ts.ViewRename("a", "c")
	if err != nil || !r.Committed || r.OldName != "a" || r.View.Name != "c" {
		t.Fatalf("rename = %+v, %v", r, err)
	}
	if got := ts.headSubject(); got != "hedgehug: view rename a c" {
		t.Errorf("subject = %q", got)
	}
	if !strings.Contains(ts.viewFile("c"), "label:a") {
		t.Errorf("renamed file = %q", ts.viewFile("c"))
	}
	_, err = ts.ViewRename("c", "B")
	wantErr(t, err, KindConflict, "view.name_taken")
	_, err = ts.ViewRename("c", "C")
	wantErr(t, err, KindConflict, "view.name_taken")
	_, err = ts.ViewRename("nope", "d")
	wantErr(t, err, KindNotFound, "view.not_found")
	_, err = ts.ViewRename("c", "d/e")
	wantErr(t, err, KindUsage, "view.bad_name")

	r, err = ts.ViewRemove("c")
	if err != nil || !r.Committed || r.View.Name != "c" {
		t.Fatalf("remove = %+v, %v", r, err)
	}
	if got := ts.headSubject(); got != "hedgehug: view rm c" {
		t.Errorf("subject = %q", got)
	}
	_, err = ts.ViewRemove("c")
	wantErr(t, err, KindNotFound, "view.not_found")
	if vs, _ := ts.ViewList(); len(vs) != 1 || vs[0].Name != "b" {
		t.Errorf("list after = %+v", vs)
	}
}

func TestViewBrokenFile(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	if _, err := ts.ViewSave("ok", ViewChange{}); err != nil {
		t.Fatal(err)
	}
	ts.writeAndCommit("views/bad.yaml", "- not a mapping\n")
	ts.writeAndCommit("views/.bad.yaml", "layout: tree\n") // a name ValidName refuses, on every system
	vs, err := ts.ViewList()
	if err != nil || len(vs) != 1 || vs[0].Name != "ok" {
		t.Errorf("list = %+v, %v", vs, err)
	}
	// .bad sorts before bad.
	if len(ts.warns) != 2 || !strings.HasPrefix(ts.warns[0], "views/.bad.yaml: ") || !strings.HasPrefix(ts.warns[1], "views/bad.yaml: does not decode: ") ||
		!strings.HasSuffix(ts.warns[1], "; run hedgehug doctor") {
		t.Errorf("warns = %q", ts.warns)
	}
	_, err = ts.ViewSave("bad", ViewChange{Query: ptr("")})
	wantErr(t, err, KindInvalid, "view.broken")
	if _, err := ts.ViewRename("bad", "worse"); err != nil {
		t.Errorf("rename of a broken view = %v", err)
	}
	if ts.viewFile("worse") != "- not a mapping\n" {
		t.Errorf("the bytes moved = %q", ts.viewFile("worse"))
	}
	if _, err := ts.ViewRemove("worse"); err != nil {
		t.Errorf("remove of a broken view = %v", err)
	}
}

func TestViewUndo(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	ts.appendConfig(aliceAccount) // as undo_test.go does
	if _, err := ts.ViewSave("a", ViewChange{Layout: ptr("table")}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.ViewSave("a", ViewChange{Query: ptr("status:todo")}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Undo(); err != nil {
		t.Fatal(err)
	}
	if v, err := ts.ViewGet("a"); err != nil || v.Query != "" || v.Layout != "table" {
		t.Errorf("after undo = %+v, %v", v, err)
	}
}

func TestViewNameTakenDetails(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	if _, err := ts.ViewSave("Release", ViewChange{}); err != nil {
		t.Fatal(err)
	}
	_, err := ts.ViewSave("release", ViewChange{})
	wantErr(t, err, KindConflict, "view.name_taken")
	if got := AsError(err).Args["existing"]; got != "Release" {
		t.Errorf("existing = %v", got)
	}
	if want := `view "release" already exists as "Release"; names that differ only in case are the same name`; err.Error() != want {
		t.Errorf("message = %q", err)
	}
}

func TestViewUndoCreateRemoveRename(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	ts.appendConfig(aliceAccount)
	exists := func(name string) bool {
		t.Helper()
		s, err := ts.OpenStore()
		if err != nil {
			t.Fatal(err)
		}
		_, err = os.Stat(filepath.Join(s.Dir(), "views", name+".yaml"))
		return err == nil
	}

	if _, err := ts.ViewSave("a", ViewChange{Query: ptr("status:todo")}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Undo(); err != nil {
		t.Fatal(err)
	}
	if exists("a") {
		t.Error("undo of a create left the file")
	}

	if _, err := ts.ViewSave("a", ViewChange{Query: ptr("status:todo")}); err != nil {
		t.Fatal(err)
	}
	before := ts.viewFile("a")
	if _, err := ts.ViewRemove("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Undo(); err != nil {
		t.Fatal(err)
	}
	if !exists("a") || ts.viewFile("a") != before {
		t.Errorf("undo of rm: exists = %v, want the file back with its content", exists("a"))
	}

	if _, err := ts.ViewRename("a", "b"); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Undo(); err != nil {
		t.Fatal(err)
	}
	if !exists("a") || exists("b") || ts.viewFile("a") != before {
		t.Errorf("undo of rename: a = %v, b = %v", exists("a"), exists("b"))
	}
}
