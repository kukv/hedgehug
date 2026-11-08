package templates_test

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/templates"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

const file = "---\nkind: epic\ntitle: Bug\n# why it is shaped so\nchildren:\n  - title: Reproduce\n    children:\n      - title: Script\n---\nSteps.\n"

func TestPutReadList(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	f, err := templates.Read(ts.Session, "bug")
	if err != nil || f.Exists || f.Content == "" {
		t.Fatalf("read of a new name = %+v, %v", f, err)
	}
	w, err := templates.Put(ts.Session, "bug", file)
	if err != nil {
		t.Fatal(err)
	}
	if w.Template.Kind != "epic" || w.Template.Children != 2 || w.Commit == nil {
		t.Fatalf("write = %+v", w)
	}
	if got := ts.HeadSubject(); !strings.Contains(got, "template-set bug") {
		t.Errorf("subject = %q", got)
	}
	f, err = templates.Read(ts.Session, "bug")
	if err != nil || !f.Exists || !strings.Contains(f.Content, "why it is shaped so") || !strings.Contains(f.Content, "Steps.") {
		t.Fatalf("read = %+v, %v", f, err)
	}
	vs, err := templates.List(ts.Session)
	if err != nil || len(vs) != 1 || vs[0].Name != "bug" || vs[0].Title != "Bug" {
		t.Fatalf("list = %+v, %v", vs, err)
	}
}

func TestPutRefusals(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	head := ts.Head()
	for _, tc := range []struct {
		name, content string
		kind          usecase.Kind
		code          string
	}{
		{"a/b", file, usecase.KindUsage, "template.bad_name"},
		{"x", "---\nkind: [\n---\n", usecase.KindInvalid, "template.invalid"},
		{"x", "---\nkind: nope\n---\n", usecase.KindInvalid, "template.invalid"},
		{"x", "---\nstatus: nope\n---\n", usecase.KindInvalid, "template.invalid"},
		{"x", "---\nchildren:\n  - title: a\n    kind: nope\n---\n", usecase.KindInvalid, "template.invalid"},
	} {
		_, err := templates.Put(ts.Session, tc.name, tc.content)
		usecasetest.WantErr(t, err, tc.kind, tc.code)
	}
	if ts.Head() != head {
		t.Error("a refused template made a commit")
	}
}

func TestRemove(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	_, err := templates.Remove(ts.Session, "bug")
	usecasetest.WantErr(t, err, usecase.KindNotFound, "template.not_found")
	if _, err := templates.Put(ts.Session, "bug", file); err != nil {
		t.Fatal(err)
	}
	if _, err := templates.Remove(ts.Session, "bug"); err != nil {
		t.Fatal(err)
	}
	if vs, _ := templates.List(ts.Session); len(vs) != 0 {
		t.Fatalf("list = %+v", vs)
	}
}

func TestRemoveRefusesWhatTheSchemaNames(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	p := ts.AddProject("p")
	for _, edit := range []func(string) string{
		func(sc string) string { return sc + "templates:\n  default:\n    task: bug\n" },
		func(sc string) string {
			return sc + "scoped:\n  - where: project:" + p.String() + "\n    templates:\n      only: [bug]\n"
		},
	} {
		_, before := ts.StoreSchema()
		if _, err := templates.Put(ts.Session, "bug", file); err != nil {
			t.Fatal(err)
		}
		ts.EditSchema(edit)
		_, err := templates.Remove(ts.Session, "bug")
		usecasetest.WantErr(t, err, usecase.KindConflict, "template.in_use")
		ts.EditSchema(func(string) string { return before })
	}
}
