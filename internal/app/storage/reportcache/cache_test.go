package reportcache

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/report"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/domain/upgrade"
	"github.com/kukv/hedgehug/internal/app/storage/history"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

var author = gitrepo.Signature{Name: "Test", Email: "test@example.com"}

type fixture struct {
	s  *store.Store
	ws model.ULID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ws := model.NewULID()
	s, err := store.Create(filepath.Join(t.TempDir(), "store"), &model.StoreMeta{ULID: model.NewULID(), Name: "test"}, nil,
		&model.Workspace{ULID: ws, Name: "me", IDPrefix: "ME", PrefixSet: time.Now()}, author, "", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{s, ws}
}

func (f *fixture) put(t *testing.T, task *model.Task) {
	t.Helper()
	if task.Workspace.IsZero() {
		task.Workspace = f.ws
	}
	if _, err := f.s.Update(store.Op{Name: "update", Author: author}, func(tx *store.Tx) error { return tx.PutTask(task) }); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) read(t *testing.T) []report.Task {
	t.Helper()
	h, err := history.Open(f.s, history.Options{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Read(f.s, h)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestReadGivesTheVersionsInOrder(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	a := &model.Task{ULID: model.NewULID(), Kind: "project", Title: "a", Status: "todo"}
	three := 3.0
	a.Points = &three
	f.put(t, a)
	a.Status = "in_progress"
	f.put(t, a)
	at := time.Now().Truncate(time.Second)
	a.Status, a.DeletedAt = "done", &at
	f.put(t, a)

	got := f.read(t)
	if len(got) != 1 || got[0].ULID != a.ULID.String() || len(got[0].Versions) != 3 {
		t.Fatalf("tasks = %+v", got)
	}
	vs := got[0].Versions
	if vs[0].Status != "todo" || vs[1].Status != "in_progress" || vs[2].Status != "done" {
		t.Fatalf("order = %+v", vs)
	}
	if !vs[0].HasPoints || vs[0].Points != 3 || vs[0].Workspace != f.ws.String() || vs[0].Deleted || !vs[2].Deleted {
		t.Fatalf("versions = %+v", vs)
	}
}

func TestReadKeepsRowsAndReadsOnlyWhatIsNew(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	a := &model.Task{ULID: model.NewULID(), Kind: "project", Title: "a", Status: "todo"}
	f.put(t, a)
	f.read(t)
	if got := Rows(f.s); got != 1 {
		t.Fatalf("rows = %d", got)
	}

	// A status only the cache has shows the row is not read again.
	c := load(f.s)
	p := model.TaskPath(a.ULID)
	c.Rows[p][0].Status = "from the cache"
	save(f.s, c)
	a.Status = "done"
	f.put(t, a)
	got := f.read(t)
	if vs := got[0].Versions; len(vs) != 2 || vs[0].Status != "from the cache" || vs[1].Status != "done" {
		t.Fatalf("versions = %+v", vs)
	}
	if got := Rows(f.s); got != 2 {
		t.Fatalf("rows = %d", got)
	}
}

func TestReadStartsOverForAnotherFormat(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	a := &model.Task{ULID: model.NewULID(), Kind: "project", Title: "a", Status: "todo"}
	f.put(t, a)
	save(f.s, file{Format: format + 1, Rows: map[string][]row{model.TaskPath(a.ULID): {{Status: "stale"}}}})
	if got := f.read(t); got[0].Versions[0].Status != "todo" {
		t.Fatalf("versions = %+v", got[0].Versions)
	}
	if err := f.s.WriteLocal(fileName, []byte("not gob")); err != nil {
		t.Fatal(err)
	}
	if got := f.read(t); got[0].Versions[0].Status != "todo" {
		t.Fatalf("versions = %+v", got[0].Versions)
	}
}

func TestReadDropsRowsOfCommitsThatAreGone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	a := &model.Task{ULID: model.NewULID(), Kind: "project", Title: "a", Status: "todo"}
	f.put(t, a)
	f.read(t)
	c := load(f.s)
	p := model.TaskPath(a.ULID)
	c.Rows[p] = append(c.Rows[p], row{Status: "of a commit no one has"})
	c.Rows["tasks/gone.md"] = []row{{}}
	save(f.s, c)
	f.read(t)
	if got := Rows(f.s); got != 1 {
		t.Fatalf("rows = %d", got)
	}
}

func TestBrokenAndForwardVersions(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	a := &model.Task{ULID: model.NewULID(), Kind: "project", Title: "a", Status: "todo"}
	f.put(t, a)
	p := model.TaskPath(a.ULID)
	if _, err := f.s.Update(store.Op{Name: "edit", Author: author}, func(tx *store.Tx) error {
		return tx.PutFile(p, []byte("---\nnot: closed"))
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.Update(store.Op{Name: "out", Author: author}, func(tx *store.Tx) error {
		return tx.PutFile(p, []byte("---\nulid: "+a.ULID.String()+"\nmoved_to: "+model.NewULID().String()+"\nmoved_at: 2026-11-05T00:00:00Z\n---\n"))
	}); err != nil {
		t.Fatal(err)
	}
	got := f.read(t)
	vs := got[0].Versions
	if len(vs) != 2 || vs[0].Gone || !vs[1].Gone {
		t.Fatalf("versions = %+v", vs)
	}

	h, err := history.Open(f.s, history.Options{})
	if err != nil {
		t.Fatal(err)
	}
	one, err := Versions(f.s, h, a.ULID)
	if err != nil || len(one.Versions) != 2 {
		t.Fatalf("Versions = %+v, %v", one, err)
	}
}

// A version from before a conversion is read in the newest format: the
// versions the commits of format 4 hold are upgraded by a step of format 5 that
// stands in for a real one. The test must not be parallel.
func TestReadUpgradesTheVersionsOfAnOlderFormat(t *testing.T) {
	f := newFixture(t)
	a := &model.Task{ULID: model.NewULID(), Kind: "project", Title: "a", Status: "todo"}
	f.put(t, a)
	a.Status = "done"
	f.put(t, a)

	was, wasSteps := model.StoreFormat, upgrade.Steps
	model.StoreFormat = 5
	upgrade.Steps = append(slices.Clone(wasSteps), upgrade.Step{From: 4, Apply: func(raw []byte, _ upgrade.Cx) ([]byte, error) {
		tk, err := model.DecodeTask(raw)
		if err != nil {
			return nil, err
		}
		if tk.Status == "todo" {
			tk.Status = "in_progress"
		}
		return tk.Encode()
	}})
	t.Cleanup(func() { model.StoreFormat, upgrade.Steps = was, wasSteps })

	vs := f.read(t)[0].Versions
	if len(vs) != 2 || vs[0].Status != "in_progress" || vs[1].Status != "done" {
		t.Fatalf("versions = %+v; want in_progress then done", vs)
	}
	h, err := history.Open(f.s, history.Options{})
	if err != nil {
		t.Fatal(err)
	}
	one, err := Versions(f.s, h, a.ULID)
	if err != nil || len(one.Versions) != 2 || one.Versions[0].Status != "in_progress" {
		t.Errorf("Versions = %+v, %v", one.Versions, err)
	}
}

// A version from a commit of format 3 keeps its points in props (the property
// of the store, here a text); the report reads it as the points of the task.
func TestReadTakesThePointsOfAVersionOfFormat3FromProps(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	dir := f.s.Dir()
	meta, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(model.StoreMetaPath))) //nolint:gosec // the test's store
	if err != nil {
		t.Fatal(err)
	}
	a := &model.Task{ULID: model.NewULID(), Kind: "project", Title: "a", Status: "todo", Workspace: f.ws}
	if err := yaml.Unmarshal([]byte("points: \"3\"\n"), &a.Props); err != nil {
		t.Fatal(err)
	}
	body, err := a.Encode()
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		model.StoreMetaPath:    strings.Replace(string(meta), fmt.Sprintf("format: %d", model.StoreFormat), "format: 3", 1),
		schema.Path:            "props:\n  points: {type: text}\n",
		model.TaskPath(a.ULID): string(body),
	}
	var paths []string
	for rel, data := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, rel)
	}
	if _, err := f.s.Repo().Commit(gitrepo.Commit{Paths: paths, Subject: "a, in format 3", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	vs := f.read(t)[0].Versions
	if len(vs) != 1 || !vs[0].HasPoints || vs[0].Points != 3 {
		t.Errorf("versions = %+v; want the points 3 of props", vs)
	}
}

// The history of a task runs through the conversion: the points of a version of
// format 3 (in props), of the commit that converts it (the field) and of a later
// one read as 3, 3 and 5, so that a burndown sees the one change that was made.
func TestReadTakesThePointsOfAHistoryThatRunsThroughTheConversion(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	dir := f.s.Dir()
	meta, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(model.StoreMetaPath))) //nolint:gosec // the test's store
	if err != nil {
		t.Fatal(err)
	}
	a := &model.Task{ULID: model.NewULID(), Kind: "project", Title: "a", Status: "todo", Workspace: f.ws}
	commit := func(format int, subject string, edit func()) {
		t.Helper()
		a.Props, a.Points = nil, nil
		edit()
		body, err := a.Encode()
		if err != nil {
			t.Fatal(err)
		}
		files := map[string]string{
			model.StoreMetaPath:    strings.Replace(string(meta), fmt.Sprintf("format: %d", model.StoreFormat), fmt.Sprintf("format: %d", format), 1),
			model.TaskPath(a.ULID): string(body),
		}
		var paths []string
		for rel, data := range files {
			p := filepath.Join(dir, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			paths = append(paths, rel)
		}
		if _, err := f.s.Repo().Commit(gitrepo.Commit{Paths: paths, Subject: subject, Author: author, When: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	points := func(v float64) func() { return func() { a.Points = &v } }
	commit(3, "in format 3", func() {
		if err := yaml.Unmarshal([]byte("points: 3\n"), &a.Props); err != nil {
			t.Fatal(err)
		}
	})
	commit(model.StoreFormat, "converted", points(3))
	commit(model.StoreFormat, "changed", points(5))
	vs := f.read(t)[0].Versions
	var got []float64
	for _, v := range vs {
		if !v.HasPoints {
			t.Fatalf("versions = %+v; every version has points", vs)
		}
		got = append(got, v.Points)
	}
	if !slices.Equal(got, []float64{3, 3, 5}) {
		t.Errorf("points = %v; want 3, 3, 5", got)
	}
}
