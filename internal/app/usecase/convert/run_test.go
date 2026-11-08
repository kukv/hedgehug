package convert

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/merge"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/replication"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// oldTasks are the tasks of the store in format 1 that the tests make, by
// title.
type oldTasks struct {
	ws                       *model.Workspace
	project, area, root      *model.Task
	story, underStory, sharp *model.Task
}

// commitRaw writes files into the store of ts and commits them, as an older
// hedgehug did: the store may be in a format that does not open.
func commitRaw(t *testing.T, ts *usecasetest.Session, subject string, files map[string]string) {
	t.Helper()
	_, e, err := ts.Entry()
	if err != nil {
		t.Fatal(err)
	}
	s, err := ts.OpenStoreAtAnyFormat(e.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Repo().Close() }()
	var paths []string
	for rel, body := range files {
		p := filepath.Join(e.Path, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, rel)
	}
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: paths, Subject: subject, Author: config.CommitAuthor(usecasetest.TestAccount), When: ts.Now()}); err != nil {
		t.Fatal(err)
	}
}

// rawTask is the file of a task of format 1 with the given parent.
func rawTask(t *testing.T, ws *model.Workspace, n int, kind, title string, parent *model.Task) (*model.Task, string) {
	t.Helper()
	tk := &model.Task{
		ULID: model.DerivedULID(ws.Created, "old "+title), ID: model.FormatTaskID(ws.IDPrefix, n), Workspace: ws.ULID,
		Kind: kind, Title: title, Status: "todo", Created: ws.Created, Updated: ws.Created,
	}
	if parent != nil {
		tk.Parent = &parent.ULID
	}
	b, err := tk.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return tk, string(b)
}

// makeOld turns the store of ts, which has one workspace, into a store in
// format 1 with some tasks, in one commit.
func makeOld(t *testing.T, ts *usecasetest.Session) *oldTasks {
	t.Helper()
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	ws, _ := ix.Workspace(ts.WorkspaceULID("me"))
	o := &oldTasks{ws: &model.Workspace{ULID: ws.ULID, Name: ws.Name, IDPrefix: ws.IDPrefix, Created: ws.Created}}
	files := map[string]string{"schema.yaml": oldSchema}
	add := func(dst **model.Task, n int, kind, title string, parent *model.Task) {
		tk, body := rawTask(t, o.ws, n, kind, title, parent)
		files[model.TaskPath(tk.ULID)] = body
		*dst = tk
	}
	add(&o.project, 1, "project", "web", nil)
	add(&o.area, 2, "area", "an area", nil)
	add(&o.root, 3, "task", "root task", nil)
	add(&o.story, 4, "story", "a story", o.project)
	add(&o.underStory, 5, "task", "task under story", o.story)
	add(&o.sharp, 6, "chore", "a chore", o.project)
	for _, name := range []string{"tmpl-task", "tmpl-bug", "tmpl-area", "tmpl-chore"} {
		b, err := (&model.Template{Name: name, Item: model.TemplateItem{Title: name}}).Encode()
		if err != nil {
			t.Fatal(err)
		}
		files[model.TemplatePath(name)] = string(b)
	}
	// ranks that tell the order: each after the one before
	var last model.Rank
	for _, tk := range []*model.Task{o.project, o.area, o.root, o.story, o.underStory, o.sharp} {
		tk.Rank, _ = model.Between(last, "")
		last = tk.Rank
		b, err := tk.Encode()
		if err != nil {
			t.Fatal(err)
		}
		files[model.TaskPath(tk.ULID)] = string(b)
	}
	meta, err := os.ReadFile(filepath.Join(storeDir(t, ts), filepath.FromSlash(model.StoreMetaPath))) //nolint:gosec // the test's store
	if err != nil {
		t.Fatal(err)
	}
	files[model.StoreMetaPath] = strings.Replace(string(meta), "format: 2", "format: 1", 1)
	commitRaw(t, ts, "make it old", files)
	ts.DropMount() // what the commands of a new run read from the files
	return o
}

func storeDir(t *testing.T, ts *usecasetest.Session) string {
	t.Helper()
	_, e, err := ts.Entry()
	if err != nil {
		t.Fatal(err)
	}
	return e.Path
}

func formatOf(t *testing.T, ts *usecasetest.Session) int {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(storeDir(t, ts), filepath.FromSlash(model.StoreMetaPath))) //nolint:gosec // the test's store
	if err != nil {
		t.Fatal(err)
	}
	m, err := model.DecodeStoreMeta(b)
	if err != nil {
		t.Fatal(err)
	}
	return m.Format
}

func headOf(t *testing.T, ts *usecasetest.Session) string {
	t.Helper()
	s, err := ts.OpenStoreAtAnyFormat(storeDir(t, ts))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Repo().Close() }()
	h, err := s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	return h.String()
}

func wantCode(t *testing.T, err error, kind usecase.Kind, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("no error, want %s", code)
	}
	e := usecase.AsError(err)
	if e.Kind != kind || e.Code != code {
		t.Fatalf("error = %v (kind %d, code %q), want kind %d, code %q", err, e.Kind, e.Code, kind, code)
	}
}

func TestOldFormatDoesNotOpen(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	makeOld(t, ts)
	_, _, err := ts.Open()
	wantCode(t, err, usecase.KindInvalid, "store.old_format")
	if got := err.Error(); got != "hh store convert で新しい形式にしてください" {
		t.Errorf("message = %q", got)
	}
}

func TestNewerFormatDoesNotOpen(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	commitRaw(t, ts, "newer", map[string]string{model.StoreMetaPath: "ulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nname: me\nformat: 3\n"})
	_, _, err := ts.Open()
	wantCode(t, err, usecase.KindInvalid, "store.newer_format")
}

func TestDoctorTellsOfAnOldFormat(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	makeOld(t, ts)
	r, err := ts.DoctorOne(false)
	if err != nil {
		t.Fatal(err)
	}
	var found *usecase.Finding
	for i, f := range r.Findings {
		if f.Code == "store.old_format" {
			found = &r.Findings[i]
		} else if f.Check != "merge_driver" { // a test has no hedgehug on PATH
			t.Errorf("another finding: %+v", f)
		}
	}
	if found == nil {
		t.Fatalf("no store.old_format finding in %+v", r.Findings)
	}
	if !strings.Contains(found.Hint, "hh store convert") {
		t.Errorf("hint = %q", found.Hint)
	}
}

func TestRunConverts(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	o := makeOld(t, ts)
	before := headOf(t, ts)
	reports, err := Run(ts.Session, Options{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || reports[0].From != 1 || len(reports[0].Lines) == 0 {
		t.Fatalf("reports = %+v", reports)
	}
	if formatOf(t, ts) != model.StoreFormat {
		t.Errorf("format = %d", formatOf(t, ts))
	}
	// one commit, of the op convert
	if usecasetest.LastOp(t, ts) != "convert" {
		t.Errorf("last op = %q", usecasetest.LastOp(t, ts))
	}
	s, ix, err := ts.Open()
	if err != nil {
		t.Fatalf("the store does not open: %v", err)
	}
	if h, _ := s.Repo().Head(); h.String() == before {
		t.Error("nothing was committed")
	}
	// the files are as the rules say
	check := func(tk *model.Task, kind string, parent *model.ULID) {
		t.Helper()
		e, ok := ix.Task(tk.ULID)
		if !ok {
			t.Fatalf("%s is gone", tk.Title)
		}
		if e.Kind != kind || (parent == nil) != (e.Parent == nil) || (parent != nil && *e.Parent != *parent) {
			t.Errorf("%s: kind %q parent %v, want %q %v", tk.Title, e.Kind, e.Parent, kind, parent)
		}
	}
	unc := model.DerivedULID(o.ws.Created, "uncategorized "+o.ws.ULID.String())
	check(o.project, "project", nil)
	check(o.area, "project", nil)
	check(o.root, "task", &unc)
	check(o.story, "story", &o.project.ULID)
	check(o.underStory, "step", &o.story.ULID)
	check(o.sharp, "task", &o.project.ULID)
	if e, ok := ix.Task(unc); !ok || e.Title != merge.Uncategorized || e.ID != "ME-7" {
		t.Errorf("未分類 = %+v, %v", e, ok)
	}
	// nothing is left to tell
	r, err := ts.DoctorOne(false)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range r.Findings {
		if f.Check != "merge_driver" {
			t.Errorf("finding after the conversion: %+v", f)
		}
	}
}

func TestRunDryRunWritesNothing(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	makeOld(t, ts)
	before := headOf(t, ts)
	reports, err := Run(ts.Session, Options{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || len(reports[0].Lines) < 3 {
		t.Errorf("reports = %+v", reports)
	}
	if headOf(t, ts) != before || formatOf(t, ts) != 1 {
		t.Error("a dry run wrote")
	}
	if st := gitStatus(t, ts); st != 0 {
		t.Errorf("%d files changed in the work tree", st)
	}
}

func gitStatus(t *testing.T, ts *usecasetest.Session) int {
	t.Helper()
	s, err := ts.OpenStoreAtAnyFormat(storeDir(t, ts))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Repo().Close() }()
	st, err := s.Repo().Status()
	if err != nil {
		t.Fatal(err)
	}
	return len(st)
}

func TestRunTwiceDoesNothing(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	makeOld(t, ts)
	if _, err := Run(ts.Session, Options{}, false); err != nil {
		t.Fatal(err)
	}
	before := headOf(t, ts)
	reports, err := Run(ts.Session, Options{}, false)
	if err != nil || len(reports) != 0 {
		t.Errorf("second run = %v, %v", reports, err)
	}
	if headOf(t, ts) != before {
		t.Error("the second run committed")
	}
	reports, err = Run(ts.Session, Options{}, true)
	if err != nil || len(reports) != 0 {
		t.Errorf("a dry run of the second = %v, %v", reports, err)
	}
}

func TestRunKindFlag(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	o := makeOld(t, ts)
	if _, err := Run(ts.Session, Options{Kinds: map[string]string{"chore": "bug"}}, false); err != nil {
		t.Fatal(err)
	}
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	if e, _ := ix.Task(o.sharp.ULID); e.Kind != "bug" {
		t.Errorf("kind = %q", e.Kind)
	}
	// a bad --kind stops before anything is written
	ts2 := usecasetest.New(t)
	makeOld(t, ts2)
	before := headOf(t, ts2)
	_, err = Run(ts2.Session, Options{Kinds: map[string]string{"chore": "sprint"}}, false)
	wantCode(t, err, usecase.KindUsage, "convert.bad_kind")
	if headOf(t, ts2) != before {
		t.Error("a bad --kind wrote")
	}
}

// remoteFx is a store in format 1 with a remote, which was pushed.
type remoteFx struct {
	ts     *usecasetest.Session
	remote string
	o      *oldTasks
}

func newRemoteFx(t *testing.T) *remoteFx {
	t.Helper()
	ts := usecasetest.New(t)
	f := &remoteFx{ts: ts, remote: usecasetest.AddBareRemote(t, ts)}
	if _, err := replication.Sync(ts.Session, false, true); err != nil {
		t.Fatal(err)
	}
	f.o = makeOld(t, ts)
	f.push(t, ts)
	return f
}

// push sends the store of ss to origin, as a sync does, though the store is
// in format 1.
func (f *remoteFx) push(t *testing.T, ss *usecasetest.Session) {
	t.Helper()
	c, e, err := ss.Entry()
	if err != nil {
		t.Fatal(err)
	}
	s, err := ss.OpenStoreAtAnyFormat(e.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Repo().Close() }()
	opt, err := ss.SyncOptions(c, e)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ss.RunSync(s, opt, true); err != nil {
		t.Fatal(err)
	}
}

func originHead(t *testing.T, remote string) string {
	t.Helper()
	r, err := gitrepo.Open(remote)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	h, err := r.ResolveCommit("refs/heads/main")
	if err != nil {
		t.Fatal(err)
	}
	return h.String()
}

func TestRunPushesWhatItConverts(t *testing.T) {
	t.Parallel()
	f := newRemoteFx(t)
	if _, err := Run(f.ts.Session, Options{}, false); err != nil {
		t.Fatal(err)
	}
	if originHead(t, f.remote) != headOf(t, f.ts) {
		t.Error("origin does not have the commit of the conversion")
	}
}

func TestRunRefusesAnUnsyncedStore(t *testing.T) {
	t.Parallel()
	// changes that are not pushed
	f := newRemoteFx(t)
	commitRaw(t, f.ts, "not pushed", map[string]string{"notes.txt": "x"})
	before := headOf(t, f.ts)
	_, err := Run(f.ts.Session, Options{}, false)
	wantCode(t, err, usecase.KindConflict, "store.convert_unsynced")
	if headOf(t, f.ts) != before || formatOf(t, f.ts) != 1 {
		t.Error("the refused conversion wrote")
	}
	// a dry run does not look at the remote
	if _, err := Run(f.ts.Session, Options{}, true); err != nil {
		t.Errorf("dry run: %v", err)
	}

	// changes of the remote that the store lacks
	g := newRemoteFx(t)
	pushElsewhere(t, g)
	_, err = Run(g.ts.Session, Options{}, false)
	wantCode(t, err, usecase.KindConflict, "store.convert_unsynced")
}

// pushElsewhere puts a commit on origin that the store of f.ts lacks, as a
// clone elsewhere would. The store of f.ts is in format 1, which hedgehug does
// not clone, so the clone is made with the repository library.
func pushElsewhere(t *testing.T, f *remoteFx) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "clone")
	r, err := gitrepo.Clone(f.remote, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("y"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Commit(gitrepo.Commit{Paths: []string{"notes.txt"}, Subject: "elsewhere", Author: config.CommitAuthor(usecasetest.TestAccount), When: f.ts.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := r.Push("origin"); err != nil {
		t.Fatal(err)
	}
}

func TestFormatDivergedAndTheWayBack(t *testing.T) {
	t.Parallel()
	a := newRemoteFx(t) // origin holds format 1
	// a second clone: it takes origin as it is, in format 1, and has a change
	// of its own that is not pushed
	b := cloneOld(t, a)
	commitRaw(t, b, "mine", map[string]string{"notes.txt": "mine"})

	// the first clone converts and pushes
	if _, err := Run(a.ts.Session, Options{}, false); err != nil {
		t.Fatal(err)
	}

	// the second can not sync: the formats differ
	c, e, err := b.Entry()
	if err != nil {
		t.Fatal(err)
	}
	s, err := b.OpenStoreAtAnyFormat(e.Path)
	if err != nil {
		t.Fatal(err)
	}
	opt, err := b.SyncOptions(c, e)
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.RunSync(s, opt, true)
	_ = s.Repo().Close()
	if !errors.Is(err, replicate.ErrFormatDiverged) {
		t.Fatalf("sync = %v, want ErrFormatDiverged", err)
	}
	wantCode(t, err, usecase.KindConflict, "sync.format_diverged")

	// hh store convert converts what it has and merges
	reports, err := Run(b.Session, Options{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 {
		t.Fatalf("reports = %+v", reports)
	}
	if formatOf(t, b) != model.StoreFormat {
		t.Errorf("format = %d", formatOf(t, b))
	}
	if originHead(t, a.remote) != headOf(t, b) {
		t.Error("the merge is not on origin")
	}
	if _, err := os.Stat(filepath.Join(storeDir(t, b), "notes.txt")); err != nil {
		t.Errorf("its own change is gone: %v", err)
	}
	// the 未分類 project is one file in both: the clones made the same one
	_, ix, err := b.Open()
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range ix.Tasks() {
		if e.Title == merge.Uncategorized {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d 未分類 projects, want 1", n)
	}
	r, err := b.DoctorOne(false)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range r.Findings {
		if f.Check != "merge_driver" {
			t.Errorf("finding after the way back: %+v", f)
		}
	}
}

func TestBehindAConvertedOriginFastForwards(t *testing.T) {
	t.Parallel()
	a := newRemoteFx(t)
	b := cloneOld(t, a)
	if _, err := Run(a.ts.Session, Options{}, false); err != nil {
		t.Fatal(err)
	}
	before := headOf(t, b)

	// a dry run says so and writes nothing
	reports, err := Run(b.Session, Options{}, true)
	if err != nil || len(reports) != 1 || !strings.Contains(strings.Join(reports[0].Lines, "\n"), "fast-forward") {
		t.Fatalf("dry run = %+v, %v", reports, err)
	}
	if headOf(t, b) != before || formatOf(t, b) != 1 {
		t.Error("the dry run wrote")
	}

	reports, err = Run(b.Session, Options{}, false)
	if err != nil || len(reports) != 1 || !strings.Contains(strings.Join(reports[0].Lines, "\n"), "fast-forward") {
		t.Fatalf("convert = %+v, %v", reports, err)
	}
	// the head is the commit of the conversion on origin: no commit of its own
	if headOf(t, b) != originHead(t, a.remote) || headOf(t, b) != headOf(t, a.ts) {
		t.Error("the clone is not at the head of origin")
	}
	if formatOf(t, b) != model.StoreFormat {
		t.Errorf("format = %d", formatOf(t, b))
	}
	if _, _, err := b.Open(); err != nil {
		t.Errorf("the store does not open: %v", err)
	}
	// and again: nothing to do
	if reports, err := Run(b.Session, Options{}, false); err != nil || len(reports) != 0 {
		t.Errorf("second run = %v, %v", reports, err)
	}
}

// cloneOld is a second clone of f's store, a Session of its own, taken while
// the store is in format 1 (git does it; StoreAdd would refuse).
func cloneOld(t *testing.T, f *remoteFx) *usecasetest.Session {
	t.Helper()
	b := usecasetest.Other(t, f.ts, f.ts.Now().Add(time.Minute))
	cfg, p, err := b.Config()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(p.StoresDir, "me")
	r, err := gitrepo.Clone(f.remote, dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = r.Close()
	if err := cfg.AddStore("me", dir, []string{f.remote}); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	return b
}
