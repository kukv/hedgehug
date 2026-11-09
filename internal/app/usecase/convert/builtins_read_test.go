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
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// A task of a commit from before the conversion, whose priority, points and the
// like are in props, is read as it is now: they are the fields of the task and
// nothing is left in props.

// formatThree is a store in format 3 with a project and the tasks x and y.
type formatThree struct {
	ws          *model.Workspace
	prj, x, y   *model.Task
	files       map[string]string
	metaFormat3 string
}

func makeFormatThree(t *testing.T, ts *usecasetest.Session) *formatThree {
	t.Helper()
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	w, _ := ix.Workspace(ts.WorkspaceULID("me"))
	f := &formatThree{ws: &model.Workspace{ULID: w.ULID, Name: w.Name, IDPrefix: w.IDPrefix, Created: w.Created}, files: map[string]string{"schema.yaml": schema3}}
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
	f.metaFormat3 = strings.Replace(string(meta), fmt.Sprintf("format: %d", model.StoreFormat), "format: 3", 1)
	return f
}

// commit writes x with the YAML props and the labels (the other files with the
// first commit) as a commit of format 3.
func (f *formatThree) commit(t *testing.T, ts *usecasetest.Session, subject, props string, labels []string, trailers []gitrepo.Trailer) string {
	t.Helper()
	f.x.Props = nil
	if err := yaml.Unmarshal([]byte(props), &f.x.Props); err != nil {
		t.Fatal(err)
	}
	f.x.Labels = labels
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
		files[model.StoreMetaPath] = f.metaFormat3
		f.files = nil
	}
	commitRawWith(t, ts, subject, files, trailers)
	ts.DropMount()
	return headOf(t, ts)
}

func TestShowAtReadsTheBuiltinsOfACommitBeforeTheConversion(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	f := makeFormatThree(t, ts)
	at := f.commit(t, ts, "x has builtins", "priority: Major\npoints: \"5\"\ndue: 2026-11-08T15:30:00+09:00\nnote: n\n", []string{"bug", "wip"}, nil)
	if _, err := Run(ts.Session, Options{}, false); err != nil {
		t.Fatal(err)
	}
	v, err := tasks.ShowAt(ts.Session, f.x.ID, at)
	if err != nil {
		t.Fatal(err)
	}
	if v.Priority == nil || *v.Priority != "major" || v.Points == nil || *v.Points != 5 || v.Due == nil || *v.Due != "2026-11-08" {
		t.Errorf("priority %v, points %v, due %v", v.Priority, v.Points, v.Due)
	}
	if len(v.Props) != 1 || v.Props["note"] == nil {
		t.Errorf("props = %v; want note alone", v.Props)
	}
	if !slices.Equal(v.Labels, []string{"bug"}) {
		t.Errorf("labels = %v; the list of the commit has bug and not wip", v.Labels)
	}
}

// A commit of format 3 usually has no labels: in its schema.yaml, which the
// store got only just before the conversion. Its labels are matched against the
// list that the store has now, and spelled as that list spells them.
func TestShowAtAndRestoreAtMatchTheLabelsOfAPastCommitToTheListOfNow(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	f := makeFormatThree(t, ts)
	f.files["schema.yaml"] = strings.Replace(schema3, "labels: [bug, ui]\n", "", 1)
	at := f.commit(t, ts, "x has labels", "note: n\n", []string{"BUG", "wip"}, nil)
	commitRawWith(t, ts, "the list of labels", map[string]string{"schema.yaml": schema3}, nil)
	ts.DropMount()
	if _, err := Run(ts.Session, Options{}, false); err != nil {
		t.Fatal(err)
	}
	v, err := tasks.ShowAt(ts.Session, f.x.ID, at)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(v.Labels, []string{"bug"}) {
		t.Errorf("show --at: labels = %v; the list of now has bug and not wip", v.Labels)
	}
	if _, err := tasks.RestoreAt(ts.Session, f.x.ID, at); err != nil {
		t.Fatal(err)
	}
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, _ := ix.Task(f.x.ULID)
	if !slices.Equal(e.Labels, []string{"bug"}) {
		t.Errorf("restore --at: labels = %v; want [bug]", e.Labels)
	}
}

func TestRestoreAtPutsBackTheBuiltinsOfACommitBeforeTheConversion(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	f := makeFormatThree(t, ts)
	at := f.commit(t, ts, "x has builtins", "priority: Major\npoints: \"5\"\nnote: n\n", nil, nil)
	if _, err := Run(ts.Session, Options{}, false); err != nil {
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
	if e.Priority != "major" || e.Points == nil || *e.Points != 5 || len(e.Props) != 1 {
		t.Errorf("after restore: priority %q, points %v, props %v", e.Priority, e.Points, e.Props)
	}
}

// Undo takes back a write of format 3 that the conversion came after: the
// priority that the write set is taken back, and none is left in props.
func TestUndoReadsTheBuiltinsOfCommitsBeforeTheConversion(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AppendConfig(usecasetest.AliceAccount)
	f := makeFormatThree(t, ts)
	f.commit(t, ts, "x without priority", "note: n\n", nil, nil)
	f.commit(t, ts, "set ME-2 priority", "priority: major\nnote: n\n", nil, []gitrepo.Trailer{
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
	if e.Priority != "" || len(e.Props) != 1 {
		t.Errorf("after the undo: priority %q, props %v", e.Priority, e.Props)
	}
}

// A past version with a value that is of no type of its field is read with the
// others moved and the value left in props under its own name; restoring it
// brings it back, and doctor tells of it.
func TestAPastVersionWithAValueOfNoTypeKeepsItInProps(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	f := makeFormatThree(t, ts)
	at := f.commit(t, ts, "x has a bad value", "points: lots\npriority: major\nnote: n\n", nil, nil)
	if _, err := Run(ts.Session, Options{DropBadValues: true}, false); err != nil {
		t.Fatal(err)
	}
	v, err := tasks.ShowAt(ts.Session, f.x.ID, at)
	if err != nil {
		t.Fatal(err)
	}
	if v.Priority == nil || *v.Priority != "major" || v.Points != nil || v.Props["points"] == nil {
		t.Errorf("priority %v, points %v, props %v", v.Priority, v.Points, v.Props)
	}
	if _, err := tasks.RestoreAt(ts.Session, f.x.ID, at); err != nil {
		t.Fatal(err)
	}
	r, err := ts.DoctorOne(false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, fd := range r.Findings {
		found = found || fd.Code == "task.legacy_field" && fd.Task == f.x.ULID.String() && fd.Args[0] == "props.points"
	}
	if !found {
		t.Errorf("doctor does not tell of props.points: %+v", r.Findings)
	}
}
