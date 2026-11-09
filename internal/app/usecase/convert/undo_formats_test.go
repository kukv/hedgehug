package convert

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// Undo reads a template of a commit before the conversion as it is now too: the
// values of its items that were task_ref properties come back as relations, and
// no props.relates is written.
func TestUndoReadsTheTemplatesOfCommitsBeforeTheConversion(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AppendConfig(usecasetest.AliceAccount)
	f := makeFormatTwo(t, ts)
	tpl := func(props string) string {
		return "---\nkind: task\ntitle: t\nprops:\n" + props + "---\n"
	}
	f.commit(t, ts, "setup", "note: n\n", nil)
	author := []gitrepo.Trailer{{Key: "Hedgehug-Op", Value: "template"}, {Key: "Hedgehug-Author", Value: "github.com/alice"}}
	commitRawWith(t, ts, "template before", map[string]string{model.TemplatePath("tpl"): tpl("  relates: [" + f.y.ULID.String() + "]\n  fixes: [" + f.prj.ULID.String() + "]\n")}, nil)
	commitRawWith(t, ts, "template after", map[string]string{model.TemplatePath("tpl"): tpl("  note: n\n")}, author)
	ts.DropMount()
	ts.AsAccount("bob")
	if _, err := Run(ts.Session, Options{}, false); err != nil {
		t.Fatal(err)
	}
	ts.AsAccount("alice")
	if res, err := tasks.Undo(ts.Session); err != nil || !res.Found {
		t.Fatalf("undo: %+v, %v", res, err)
	}
	b, err := os.ReadFile(filepath.Join(storeDir(t, ts), filepath.FromSlash(model.TemplatePath("tpl")))) //nolint:gosec // the test's store
	if err != nil {
		t.Fatal(err)
	}
	tp, err := model.DecodeTemplate("tpl", b)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(tp.Item.Relates, []model.ULID{f.y.ULID, f.prj.ULID}) {
		t.Errorf("relates = %v", tp.Item.Relates)
	}
	if len(tp.Item.Props) != 0 {
		t.Errorf("props = %v after the undo; a relation in props is read by nothing", tp.Item.Props)
	}
}

// Undo does not put back a schema.yaml of a commit before the conversion: it
// has the task_ref properties that the schema no longer reads.
func TestUndoRefusesToRestoreTheSchemaOfBeforeTheConversion(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AppendConfig(usecasetest.AliceAccount)
	f := makeFormatTwo(t, ts)
	f.commit(t, ts, "setup", "note: n\n", nil)
	changed := strings.Replace(schema2, "  note: {type: text}\n", "  note: {type: text}\n  extra: {type: text}\n", 1)
	commitRawWith(t, ts, "schema changed", map[string]string{"schema.yaml": changed}, []gitrepo.Trailer{
		{Key: "Hedgehug-Op", Value: "schema"}, {Key: "Hedgehug-Author", Value: "github.com/alice"},
	})
	ts.DropMount()
	ts.AsAccount("bob")
	if _, err := Run(ts.Session, Options{}, false); err != nil {
		t.Fatal(err)
	}
	ts.AsAccount("alice")
	_, err := tasks.Undo(ts.Session)
	wantCode(t, err, usecase.KindConflict, "undo.unsupported")
}

// Nor does it put back a view of a commit before the conversion: its query and
// columns use the names of format 2.
func TestUndoRefusesToRestoreAViewOfBeforeTheConversion(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AppendConfig(usecasetest.AliceAccount)
	f := makeFormatTwo(t, ts)
	f.commit(t, ts, "setup", "note: n\n", nil)
	commitRawWith(t, ts, "view before", map[string]string{model.ViewPath("v"): "layout: table\nquery: fixes:ME-1\n"}, nil)
	commitRawWith(t, ts, "view after", map[string]string{model.ViewPath("v"): "layout: table\nquery: status:todo\n"}, []gitrepo.Trailer{
		{Key: "Hedgehug-Op", Value: "view"}, {Key: "Hedgehug-Author", Value: "github.com/alice"},
	})
	ts.DropMount()
	ts.AsAccount("bob")
	if _, err := Run(ts.Session, Options{}, false); err != nil {
		t.Fatal(err)
	}
	ts.AsAccount("alice")
	_, err := tasks.Undo(ts.Session)
	wantCode(t, err, usecase.KindConflict, "undo.unsupported")
}
