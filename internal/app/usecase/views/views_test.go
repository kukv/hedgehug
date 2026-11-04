package views_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/storage/view"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/app/usecase/views"
)

func ptr[T any](v T) *T { return &v }

func TestViewSaveCreateAndOverwrite(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	r, err := views.ViewSave(ts.Session, "今週", views.ViewChange{})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Committed || r.Commit == "" || r.View.Layout != "tree" || r.View.Query != "" {
		t.Errorf("create = %+v %+v", r, r.View)
	}
	if got := ts.HeadSubject(); got != "hedgehug: view save 今週" {
		t.Errorf("subject = %q", got)
	}
	if got, _ := ts.HeadTrailer("Hedgehug-Op"); got != "view" {
		t.Errorf("Hedgehug-Op = %q", got)
	}
	if got := ts.ViewFile("今週"); got != "layout: tree\nquery: \"\"\n" {
		t.Errorf("file = %q", got)
	}
	r, err = views.ViewSave(ts.Session, "今週", views.ViewChange{Layout: ptr("table"), Columns: ptr([]string{"id", "title", "due"})})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Committed || r.View.Layout != "table" || !slices.Equal(r.View.Table.Columns, []string{"id", "title", "due"}) {
		t.Errorf("overwrite = %+v %+v", r, r.View)
	}
	// Only the flags given change; a save that changes nothing commits nothing.
	r, err = views.ViewSave(ts.Session, "今週", views.ViewChange{Query: ptr("status:todo")})
	if err != nil || r.View.Layout != "table" || r.View.Query != "status:todo" {
		t.Fatalf("query only = %+v, %v", r, err)
	}
	r, err = views.ViewSave(ts.Session, "今週", views.ViewChange{Query: ptr("status:todo")})
	if err != nil || r.Committed || r.Commit != "" {
		t.Errorf("same again = %+v, %v", r, err)
	}
}

func TestViewSaveKeepsUnknownKeys(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := views.ViewSave(ts.Session, "a", views.ViewChange{Layout: ptr("table")}); err != nil {
		t.Fatal(err)
	}
	ts.WriteAndCommit("views/a.yaml", "# note\nlayout: table\nquery: \"\"\ncolor: red\n")
	if _, err := views.ViewSave(ts.Session, "a", views.ViewChange{Query: ptr("status:todo")}); err != nil {
		t.Fatal(err)
	}
	got := ts.ViewFile("a")
	for _, w := range []string{"# note", "color: red", "status:todo"} {
		if !strings.Contains(got, w) {
			t.Errorf("file lacks %q:\n%s", w, got)
		}
	}
}

func TestViewSaveRefuses(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := views.ViewSave(ts.Session, "Release", views.ViewChange{}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		ch   views.ViewChange
		kind usecase.Kind
		code string
	}{
		{"a/b", views.ViewChange{}, usecase.KindUsage, "view.bad_name"},
		{"CON", views.ViewChange{}, usecase.KindUsage, "view.bad_name"},
		{"release", views.ViewChange{}, usecase.KindConflict, "view.name_taken"},
		{"x", views.ViewChange{Layout: ptr("gantt")}, usecase.KindUsage, "view.bad_value"},
		{"x", views.ViewChange{Layout: ptr("table"), GroupBy: ptr("owner")}, usecase.KindUsage, "view.bad_value"},
		{"x", views.ViewChange{KanbanColumns: ptr("kind")}, usecase.KindUsage, "view.bad_value"},
		{"x", views.ViewChange{Swimlane: ptr("label")}, usecase.KindUsage, "view.bad_value"},
		{"x", views.ViewChange{Columns: ptr([]string{"id", "id"})}, usecase.KindUsage, "view.bad_value"},
		{"x", views.ViewChange{Columns: ptr([]string{"id", "nope"})}, usecase.KindInvalid, "view.bad_column"},
		{"x", views.ViewChange{Query: ptr("status:todo sort:due")}, usecase.KindUsage, "view.tree_order"},
		{"x", views.ViewChange{Layout: ptr("tree"), Query: ptr("group:status")}, usecase.KindUsage, "view.tree_order"},
		{"x", views.ViewChange{Query: ptr(`"abc`)}, usecase.KindUsage, "query.invalid"},
	} {
		_, err := views.ViewSave(ts.Session, c.name, c.ch)
		usecasetest.WantErr(t, err, c.kind, c.code)
	}
	// A table may sort; switching it to tree then is refused.
	if _, err := views.ViewSave(ts.Session, "t", views.ViewChange{Layout: ptr("table"), Query: ptr("sort:due")}); err != nil {
		t.Fatal(err)
	}
	_, err := views.ViewSave(ts.Session, "t", views.ViewChange{Layout: ptr("tree")})
	usecasetest.WantErr(t, err, usecase.KindUsage, "view.tree_order")
	if err.Error() != "a tree view orders tasks by rank; its query takes neither group: nor sort:" {
		t.Errorf("message = %q", err)
	}
}

func TestViewListGetRemoveRename(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if vs, err := views.ViewList(ts.Session); err != nil || len(vs) != 0 {
		t.Fatalf("empty list = %v, %v", vs, err)
	}
	for _, n := range []string{"b", "a"} {
		if _, err := views.ViewSave(ts.Session, n, views.ViewChange{Layout: ptr("kanban"), Query: ptr("label:" + n)}); err != nil {
			t.Fatal(err)
		}
	}
	vs, err := views.ViewList(ts.Session)
	if err != nil {
		t.Fatal(err)
	}
	want := []views.SavedView{
		{Name: "a", Layout: "kanban", Query: "label:a", Table: views.SavedTable{Columns: view.DefaultColumns, GroupBy: "none"}, Kanban: views.SavedKanban{Columns: "status", Swimlane: "none"}},
		{Name: "b", Layout: "kanban", Query: "label:b", Table: views.SavedTable{Columns: view.DefaultColumns, GroupBy: "none"}, Kanban: views.SavedKanban{Columns: "status", Swimlane: "none"}},
	}
	if len(vs) != 2 || vs[0].Name != want[0].Name || vs[1].Query != want[1].Query || !slices.Equal(vs[0].Table.Columns, want[0].Table.Columns) || vs[0].Kanban != want[0].Kanban {
		t.Errorf("list = %+v", vs)
	}
	v, err := views.ViewGet(ts.Session, "a")
	if err != nil || v.Query != "label:a" {
		t.Errorf("get = %+v, %v", v, err)
	}
	_, err = views.ViewGet(ts.Session, "A")
	usecasetest.WantErr(t, err, usecase.KindNotFound, "view.not_found")

	r, err := views.ViewRename(ts.Session, "a", "c")
	if err != nil || !r.Committed || r.OldName != "a" || r.View.Name != "c" {
		t.Fatalf("rename = %+v, %v", r, err)
	}
	if got := ts.HeadSubject(); got != "hedgehug: view rename a c" {
		t.Errorf("subject = %q", got)
	}
	if !strings.Contains(ts.ViewFile("c"), "label:a") {
		t.Errorf("renamed file = %q", ts.ViewFile("c"))
	}
	_, err = views.ViewRename(ts.Session, "c", "B")
	usecasetest.WantErr(t, err, usecase.KindConflict, "view.name_taken")
	_, err = views.ViewRename(ts.Session, "c", "C")
	usecasetest.WantErr(t, err, usecase.KindConflict, "view.name_taken")
	_, err = views.ViewRename(ts.Session, "nope", "d")
	usecasetest.WantErr(t, err, usecase.KindNotFound, "view.not_found")
	_, err = views.ViewRename(ts.Session, "c", "d/e")
	usecasetest.WantErr(t, err, usecase.KindUsage, "view.bad_name")

	r, err = views.ViewRemove(ts.Session, "c")
	if err != nil || !r.Committed || r.View.Name != "c" {
		t.Fatalf("remove = %+v, %v", r, err)
	}
	if got := ts.HeadSubject(); got != "hedgehug: view rm c" {
		t.Errorf("subject = %q", got)
	}
	_, err = views.ViewRemove(ts.Session, "c")
	usecasetest.WantErr(t, err, usecase.KindNotFound, "view.not_found")
	if vs, _ := views.ViewList(ts.Session); len(vs) != 1 || vs[0].Name != "b" {
		t.Errorf("list after = %+v", vs)
	}
}

func TestViewBrokenFile(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := views.ViewSave(ts.Session, "ok", views.ViewChange{}); err != nil {
		t.Fatal(err)
	}
	ts.WriteAndCommit("views/bad.yaml", "- not a mapping\n")
	ts.WriteAndCommit("views/.bad.yaml", "layout: tree\n") // a name ValidName refuses, on every system
	vs, err := views.ViewList(ts.Session)
	if err != nil || len(vs) != 1 || vs[0].Name != "ok" {
		t.Errorf("list = %+v, %v", vs, err)
	}
	// .bad sorts before bad.
	if len(ts.Warns) != 2 || !strings.HasPrefix(ts.Warns[0], "views/.bad.yaml: ") || !strings.HasPrefix(ts.Warns[1], "views/bad.yaml: does not decode: ") ||
		!strings.HasSuffix(ts.Warns[1], "; run hedgehug doctor") {
		t.Errorf("warns = %q", ts.Warns)
	}
	_, err = views.ViewSave(ts.Session, "bad", views.ViewChange{Query: ptr("")})
	usecasetest.WantErr(t, err, usecase.KindInvalid, "view.broken")
	if _, err := views.ViewRename(ts.Session, "bad", "worse"); err != nil {
		t.Errorf("rename of a broken view = %v", err)
	}
	if ts.ViewFile("worse") != "- not a mapping\n" {
		t.Errorf("the bytes moved = %q", ts.ViewFile("worse"))
	}
	if _, err := views.ViewRemove(ts.Session, "worse"); err != nil {
		t.Errorf("remove of a broken view = %v", err)
	}
}

func TestViewUndo(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AppendConfig(usecasetest.AliceAccount) // as undo_test.go does
	if _, err := views.ViewSave(ts.Session, "a", views.ViewChange{Layout: ptr("table")}); err != nil {
		t.Fatal(err)
	}
	if _, err := views.ViewSave(ts.Session, "a", views.ViewChange{Query: ptr("status:todo")}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Undo(ts.Session); err != nil {
		t.Fatal(err)
	}
	if v, err := views.ViewGet(ts.Session, "a"); err != nil || v.Query != "" || v.Layout != "table" {
		t.Errorf("after undo = %+v, %v", v, err)
	}
}

func TestViewNameTakenDetails(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := views.ViewSave(ts.Session, "Release", views.ViewChange{}); err != nil {
		t.Fatal(err)
	}
	_, err := views.ViewSave(ts.Session, "release", views.ViewChange{})
	usecasetest.WantErr(t, err, usecase.KindConflict, "view.name_taken")
	if got := usecase.AsError(err).Args["existing"]; got != "Release" {
		t.Errorf("existing = %v", got)
	}
	if want := `view "release" already exists as "Release"; names that differ only in case are the same name`; err.Error() != want {
		t.Errorf("message = %q", err)
	}
}

func TestViewUndoCreateRemoveRename(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AppendConfig(usecasetest.AliceAccount)
	exists := func(name string) bool {
		t.Helper()
		s, err := ts.OpenStore()
		if err != nil {
			t.Fatal(err)
		}
		_, err = os.Stat(filepath.Join(s.Dir(), "views", name+".yaml"))
		return err == nil
	}

	if _, err := views.ViewSave(ts.Session, "a", views.ViewChange{Query: ptr("status:todo")}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Undo(ts.Session); err != nil {
		t.Fatal(err)
	}
	if exists("a") {
		t.Error("undo of a create left the file")
	}

	if _, err := views.ViewSave(ts.Session, "a", views.ViewChange{Query: ptr("status:todo")}); err != nil {
		t.Fatal(err)
	}
	before := ts.ViewFile("a")
	if _, err := views.ViewRemove(ts.Session, "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Undo(ts.Session); err != nil {
		t.Fatal(err)
	}
	if !exists("a") || ts.ViewFile("a") != before {
		t.Errorf("undo of rm: exists = %v, want the file back with its content", exists("a"))
	}

	if _, err := views.ViewRename(ts.Session, "a", "b"); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Undo(ts.Session); err != nil {
		t.Fatal(err)
	}
	if !exists("a") || exists("b") || ts.ViewFile("a") != before {
		t.Errorf("undo of rename: a = %v, b = %v", exists("a"), exists("b"))
	}
}

func TestViewUpdateChangesOnlyWhatIsGiven(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := views.ViewSave(ts.Session, "w", views.ViewChange{Layout: ptr("table"), Query: ptr("status:todo"), Columns: ptr([]string{"id", "title"})}); err != nil {
		t.Fatal(err)
	}
	r, err := views.ViewUpdate(ts.Session, "w", views.ViewChange{Layout: ptr("kanban")})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Committed || r.View.Layout != "kanban" || r.View.Query != "status:todo" {
		t.Errorf("update = %+v %+v", r, r.View)
	}
	if got := ts.HeadSubject(); got != "hedgehug: view update w" {
		t.Errorf("subject = %q", got)
	}
	if got := r.View.Table.Columns; !slices.Equal(got, []string{"id", "title"}) {
		t.Errorf("columns = %v", got)
	}
}

func TestViewUpdateNeverCreates(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	_, err := views.ViewUpdate(ts.Session, "gone", views.ViewChange{Layout: ptr("table")})
	usecasetest.WantErr(t, err, usecase.KindNotFound, "view.not_found")
	if _, err := views.ViewGet(ts.Session, "gone"); err == nil {
		t.Error("the view was created")
	}
	if _, err := views.ViewSave(ts.Session, "Release", views.ViewChange{}); err != nil {
		t.Fatal(err)
	}
	// A name that differs only in case is another name here: the view is not found.
	_, err = views.ViewUpdate(ts.Session, "release", views.ViewChange{Layout: ptr("table")})
	usecasetest.WantErr(t, err, usecase.KindNotFound, "view.not_found")
}

func TestViewUpdateChecksTheViewThatResults(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := views.ViewSave(ts.Session, "t", views.ViewChange{Layout: ptr("table"), Query: ptr("sort:due")}); err != nil {
		t.Fatal(err)
	}
	_, err := views.ViewUpdate(ts.Session, "t", views.ViewChange{Layout: ptr("tree")})
	usecasetest.WantErr(t, err, usecase.KindUsage, "view.tree_order")
	_, err = views.ViewUpdate(ts.Session, "t", views.ViewChange{Layout: ptr("gantt")})
	usecasetest.WantErr(t, err, usecase.KindUsage, "view.bad_value")
	_, err = views.ViewUpdate(ts.Session, "t", views.ViewChange{Query: ptr(`"abc`)})
	usecasetest.WantErr(t, err, usecase.KindUsage, "query.invalid")
	_, err = views.ViewUpdate(ts.Session, "t", views.ViewChange{Columns: ptr([]string{"id", "nope"})})
	usecasetest.WantErr(t, err, usecase.KindInvalid, "view.bad_column")
}
