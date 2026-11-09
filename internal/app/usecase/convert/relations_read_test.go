package convert

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// A task of a commit from before the conversion, whose relations are in props,
// is read as it is now: the relations are the fields of the task and nothing is
// left in props.

// formatTwo is a store in format 2 with a project and the tasks x and y.
type formatTwo struct {
	ws          *model.Workspace
	prj, x, y   *model.Task
	files       map[string]string
	metaFormat2 string
}

func makeFormatTwo(t *testing.T, ts *usecasetest.Session) *formatTwo {
	t.Helper()
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	w, _ := ix.Workspace(ts.WorkspaceULID("me"))
	f := &formatTwo{ws: &model.Workspace{ULID: w.ULID, Name: w.Name, IDPrefix: w.IDPrefix, Created: w.Created}, files: map[string]string{"schema.yaml": schema2}}
	var body string
	f.prj, body = rawTask(t, f.ws, 1, "project", "web", nil)
	f.files[model.TaskPath(f.prj.ULID)] = body
	f.x, body = rawTask(t, f.ws, 2, "task", "x", f.prj)
	f.files[model.TaskPath(f.x.ULID)] = body
	f.y, body = rawTask(t, f.ws, 3, "task", "y", f.prj)
	f.files[model.TaskPath(f.y.ULID)] = body
	meta, err := os.ReadFile(filepath.Join(storeDir(t, ts), filepath.FromSlash(model.StoreMetaPath))) //nolint:gosec // the test's store
	if err != nil {
		t.Fatal(err)
	}
	f.metaFormat2 = strings.Replace(string(meta), fmt.Sprintf("format: %d", model.StoreFormat), "format: 2", 1)
	return f
}

// commit writes x with the YAML props (the other files with the first commit)
// as a commit of format 2.
func (f *formatTwo) commit(t *testing.T, ts *usecasetest.Session, subject, props string, trailers []gitrepo.Trailer) string {
	t.Helper()
	if err := yaml.Unmarshal([]byte(props), &f.x.Props); err != nil {
		t.Fatal(err)
	}
	b, err := f.x.Encode()
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{model.TaskPath(f.x.ULID): string(b)}
	if f.files != nil {
		for p, c := range f.files {
			if p != model.TaskPath(f.x.ULID) {
				files[p] = c
			}
		}
		files[model.StoreMetaPath] = f.metaFormat2
		f.files = nil
	}
	commitRawWith(t, ts, subject, files, trailers)
	ts.DropMount()
	return headOf(t, ts)
}

func refIDs(rs []usecase.Ref) []string {
	var out []string
	for _, r := range rs {
		out = append(out, r.ULID)
	}
	return out
}

func notInProps(t *testing.T, e interface{ Prop(string) []string }) {
	t.Helper()
	for _, k := range []string{"relates", "causes", "fixes", "duplicates"} {
		if e.Prop(k) != nil {
			t.Errorf("props.%s is left: a relation in props is read by nothing", k)
		}
	}
}

func TestShowAtReadsTheRelationsOfACommitBeforeTheConversion(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	f := makeFormatTwo(t, ts)
	at := f.commit(t, ts, "x has relations", "relates: ["+f.y.ULID.String()+"]\ncauses: "+f.prj.ULID.String()+"\nfixes: ["+f.y.ULID.String()+"]\nnote: n\n", nil)
	if _, err := Run(ts.Session, Options{}, false); err != nil {
		t.Fatal(err)
	}
	v, err := tasks.ShowAt(ts.Session, f.x.ID, at)
	if err != nil {
		t.Fatal(err)
	}
	if got := refIDs(v.Relates); !slices.Equal(got, []string{f.y.ULID.String()}) {
		t.Errorf("relates = %v; the fixes of format 2 are relates too, once", got)
	}
	if got := refIDs(v.Causes); !slices.Equal(got, []string{f.prj.ULID.String()}) {
		t.Errorf("causes = %v", got)
	}
	if len(v.Props) != 1 || v.Props["note"] == nil {
		t.Errorf("props = %v; want note alone", v.Props)
	}
}

func TestRestoreAtPutsBackTheRelationsOfACommitBeforeTheConversion(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	f := makeFormatTwo(t, ts)
	at := f.commit(t, ts, "x has relations", "relates: ["+f.y.ULID.String()+"]\ncauses: "+f.prj.ULID.String()+"\nnote: n\n", nil)
	if _, err := Run(ts.Session, Options{}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Set(ts.Session, f.x.ID, []usecase.Assign{{Key: "relates", Op: "=", Value: ""}, {Key: "causes", Op: "=", Value: ""}}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.RestoreAt(ts.Session, f.x.ID, at); err != nil {
		t.Fatal(err)
	}
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, _ := ix.Task(f.x.ULID)
	if !slices.Equal(e.Relates, []model.ULID{f.y.ULID}) || !slices.Equal(e.Causes, []model.ULID{f.prj.ULID}) {
		t.Errorf("after restore: relates %v, causes %v", e.Relates, e.Causes)
	}
	notInProps(t, e)
}

// Undo takes back a write of format 2 that the conversion came after: the
// relation that the write added is taken back, and none is left in props.
func TestUndoReadsTheRelationsOfCommitsBeforeTheConversion(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AppendConfig(usecasetest.AliceAccount)
	f := makeFormatTwo(t, ts)
	f.commit(t, ts, "x without relations", "note: n\n", nil)
	f.commit(t, ts, "set ME-2 relates", "relates: ["+f.y.ULID.String()+"]\nnote: n\n", []gitrepo.Trailer{
		{Key: "Hedgehug-Op", Value: "set"}, {Key: "Hedgehug-Author", Value: "github.com/alice"},
	})
	ts.AsAccount("bob")
	if _, err := Run(ts.Session, Options{}, false); err != nil {
		t.Fatal(err)
	}
	ts.AsAccount("alice")
	res, err := tasks.Undo(ts.Session)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Found || len(res.Kept) != 0 {
		t.Errorf("found %v, kept %+v; want found, nothing kept", res.Found, res.Kept)
	}
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, _ := ix.Task(f.x.ULID)
	if len(e.Relates) != 0 {
		t.Errorf("relates = %v after the undo", e.Relates)
	}
	notInProps(t, e)
}

// A past version with a value that is no ULID is read with the ULIDs moved and
// the value left in props under its own name; restoring it brings it back, and
// doctor tells of it.
func TestAPastVersionWithAValueThatIsNoULIDKeepsItInProps(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	f := makeFormatTwo(t, ts)
	at := f.commit(t, ts, "x has a bad value", "relates: ["+f.y.ULID.String()+", nope]\nnote: n\n", nil)
	if _, err := Run(ts.Session, Options{DropBadValues: true}, false); err != nil {
		t.Fatal(err)
	}
	v, err := tasks.ShowAt(ts.Session, f.x.ID, at)
	if err != nil {
		t.Fatal(err)
	}
	if got := refIDs(v.Relates); !slices.Equal(got, []string{f.y.ULID.String()}) {
		t.Errorf("relates = %v", got)
	}
	if v.Props["relates"] == nil {
		t.Errorf("props = %v; the value that is no ULID is not there", v.Props)
	}
	if _, err := tasks.RestoreAt(ts.Session, f.x.ID, at); err != nil {
		t.Fatal(err)
	}
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, _ := ix.Task(f.x.ULID)
	if !slices.Equal(e.Relates, []model.ULID{f.y.ULID}) || !slices.Equal(e.Prop("relates"), []string{"nope"}) {
		t.Errorf("after restore: relates %v, props.relates %v", e.Relates, e.Prop("relates"))
	}
	r, err := ts.DoctorOne(false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, fd := range r.Findings {
		found = found || fd.Code == "task.legacy_field" && fd.Task == f.x.ULID.String() && fd.Args[0] == "props.relates"
	}
	if !found {
		t.Errorf("doctor does not tell of props.relates: %+v", r.Findings)
	}
}
