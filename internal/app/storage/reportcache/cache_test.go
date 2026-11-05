package reportcache

import (
	"path/filepath"
	"testing"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/report"
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
	a := &model.Task{ULID: model.NewULID(), Kind: "task", Title: "a", Status: "todo"}
	a.Props.Set("points", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "3"})
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
	a := &model.Task{ULID: model.NewULID(), Kind: "task", Title: "a", Status: "todo"}
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
	a := &model.Task{ULID: model.NewULID(), Kind: "task", Title: "a", Status: "todo"}
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
	a := &model.Task{ULID: model.NewULID(), Kind: "task", Title: "a", Status: "todo"}
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
	a := &model.Task{ULID: model.NewULID(), Kind: "task", Title: "a", Status: "todo"}
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
