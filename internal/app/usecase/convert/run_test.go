package convert

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
	commitRawWith(t, ts, subject, files, nil)
}

// commitRawWith is commitRaw with the trailers of the commit.
func commitRawWith(t *testing.T, ts *usecasetest.Session, subject string, files map[string]string, trailers []gitrepo.Trailer) {
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
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: paths, Subject: subject, Trailers: trailers, Author: config.CommitAuthor(usecasetest.TestAccount), When: ts.Now()}); err != nil {
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
	files[model.StoreMetaPath] = strings.Replace(string(meta), fmt.Sprintf("format: %d", model.StoreFormat), "format: 1", 1)
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
	commitRaw(t, ts, "newer", map[string]string{model.StoreMetaPath: "ulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nname: me\nformat: 5\n"})
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
	if len(reports) != 3 || reports[0].From != 1 || reports[1].From != 2 || len(reports[0].Lines) == 0 {
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
	if len(reports) != 3 || len(reports[0].Lines) < 3 {
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

// wantConvertedAndPushed checks that ts is converted, that origin has exactly its head, and that the
// files are in its working tree.
func wantConvertedAndPushed(t *testing.T, f *remoteFx, ts *usecasetest.Session, files ...string) {
	t.Helper()
	if formatOf(t, ts) != model.StoreFormat {
		t.Errorf("format = %d", formatOf(t, ts))
	}
	if originHead(t, f.remote) != headOf(t, ts) {
		t.Error("origin is not at the head of the store")
	}
	for _, rel := range files {
		if _, err := os.Stat(filepath.Join(storeDir(t, ts), filepath.FromSlash(rel))); err != nil {
			t.Errorf("%s was lost: %v", rel, err)
		}
	}
}

// An unsynced store whose origin is in the same format is synced in that format first, and then
// converted and pushed, by the one command.
func TestRunSyncsFirstInTheSameFormat(t *testing.T) {
	t.Parallel()
	t.Run("changes not pushed", func(t *testing.T) {
		t.Parallel()
		f := newRemoteFx(t)
		commitRaw(t, f.ts, "not pushed", map[string]string{"notes.txt": "x"})
		if _, err := Run(f.ts.Session, Options{}, false); err != nil {
			t.Fatal(err)
		}
		wantConvertedAndPushed(t, f, f.ts, "notes.txt")
	})
	t.Run("changes of origin the store lacks", func(t *testing.T) {
		t.Parallel()
		f := newRemoteFx(t)
		pushElsewhere(t, f)
		if _, err := Run(f.ts.Session, Options{}, false); err != nil {
			t.Fatal(err)
		}
		wantConvertedAndPushed(t, f, f.ts, "notes.txt")
	})
	t.Run("both", func(t *testing.T) {
		t.Parallel()
		f := newRemoteFx(t)
		pushFile(t, f, "remote.txt", "r")
		commitRaw(t, f.ts, "not pushed", map[string]string{"local.txt": "l"})
		if _, err := Run(f.ts.Session, Options{}, false); err != nil {
			t.Fatal(err)
		}
		wantConvertedAndPushed(t, f, f.ts, "remote.txt", "local.txt")
	})
	t.Run("a dry run says it and writes nothing", func(t *testing.T) {
		t.Parallel()
		f := newRemoteFx(t)
		commitRaw(t, f.ts, "not pushed", map[string]string{"notes.txt": "x"})
		before, remote := headOf(t, f.ts), originHead(t, f.remote)
		reports, err := Run(f.ts.Session, Options{}, true)
		if err != nil || len(reports) == 0 || !strings.Contains(strings.Join(reports[0].Lines, "\n"), "先に古い形式のまま") {
			t.Fatalf("dry run = %+v, %v", reports, err)
		}
		if headOf(t, f.ts) != before || originHead(t, f.remote) != remote || formatOf(t, f.ts) != 1 {
			t.Error("the dry run wrote")
		}
	})
}

// The sync that comes before the conversion can fail: the store is then not converted, and the
// error says why.
func TestRunSyncFailureIsConvertUnsynced(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes where a mode forbids it")
	}
	f := newRemoteFx(t)
	commitRaw(t, f.ts, "not pushed", map[string]string{"notes.txt": "x"})
	remote := originHead(t, f.remote)
	// origin can be read and not written: the push fails
	var dirs []string
	_ = filepath.WalkDir(f.remote, func(p string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			dirs = append(dirs, p)
		}
		return nil
	})
	slices.Reverse(dirs) // children first: the mode of a parent does not hide them
	for _, d := range dirs {
		if err := os.Chmod(d, 0o500); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, d := range dirs {
			_ = os.Chmod(d, 0o700)
		}
	})
	_, err := Run(f.ts.Session, Options{}, false)
	wantCode(t, err, usecase.KindConflict, "store.convert_unsynced")
	if !strings.Contains(err.Error(), "the sync before the conversion failed") || strings.TrimSpace(strings.SplitN(err.Error(), ": ", 2)[1]) == "" {
		t.Errorf("the error does not tell the cause: %v", err)
	}
	t.Log(err)
	for _, d := range dirs {
		_ = os.Chmod(d, 0o700)
	}
	if formatOf(t, f.ts) != 1 {
		t.Errorf("the store was converted: format %d", formatOf(t, f.ts))
	}
	if originHead(t, f.remote) != remote {
		t.Error("origin changed")
	}
}

// An origin that is in an older format than the store has its changes converted on origin, up to the format of the store.
func TestRunOriginOlderThanTheStore(t *testing.T) {
	f := newRemoteFx(t) // origin and store in format 1
	withFormat5(t)
	meta := filepath.Join(storeDir(t, f.ts), filepath.FromSlash(model.StoreMetaPath))
	b, err := os.ReadFile(meta)
	if err != nil {
		t.Fatal(err)
	}
	commitRaw(t, f.ts, "a store in format 4", map[string]string{model.StoreMetaPath: strings.Replace(string(b), "format: 1", "format: 4", 1)})
	pushElsewhere(t, f)
	reports, err := Run(f.ts.Session, Options{}, false)
	if err != nil {
		t.Fatal(err)
	}
	// the changes of origin are converted from 1 up to the format of the
	// store (4), and then the store goes on to 5
	if len(reports) != 5 || reports[0].From != 1 || reports[0].To != 4 || reports[4].From != 4 || reports[4].To != 5 {
		t.Fatalf("reports = %+v", reports)
	}
	if formatOf(t, f.ts) != 5 || originHead(t, f.remote) != headOf(t, f.ts) {
		t.Errorf("format %d; origin is not at the head of the store", formatOf(t, f.ts))
	}
	if _, err := os.Stat(filepath.Join(storeDir(t, f.ts), "notes.txt")); err != nil {
		t.Error(err)
	}
}

// pushElsewhere puts a commit on origin that the store of f.ts lacks, as a
// clone elsewhere would. The store of f.ts is in format 1, which hedgehug does
// not clone, so the clone is made with the repository library.
func pushElsewhere(t *testing.T, f *remoteFx) { pushFile(t, f, "notes.txt", "y") }

// pushFile is pushElsewhere with the file it commits.
func pushFile(t *testing.T, f *remoteFx, rel, body string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "clone")
	r, err := gitrepo.Clone(f.remote, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, filepath.FromSlash(rel))), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Commit(gitrepo.Commit{Paths: []string{rel}, Subject: "elsewhere", Author: config.CommitAuthor(usecasetest.TestAccount), When: f.ts.Now()}); err != nil {
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
	// including a task at the root, which goes under 未分類, and takes the
	// number the other clone gives to 未分類
	mine, _ := rawTask(t, a.o.ws, 7, "task", "mine at the root", nil)
	mine.Rank, _ = model.Between("", "")
	enc, err := mine.Encode()
	if err != nil {
		t.Fatal(err)
	}
	commitRaw(t, b, "mine", map[string]string{"notes.txt": "mine", model.TaskPath(mine.ULID): string(enc)})

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
	if len(reports) != 3 {
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
	ids := map[string]bool{}
	var unc model.ULID
	for _, e := range ix.Tasks() {
		if e.Title == merge.Uncategorized {
			n++
			unc = e.ULID
		}
		if ids[e.ID] {
			t.Errorf("the id %s is on two tasks", e.ID)
		}
		ids[e.ID] = true
	}
	if n != 1 {
		t.Errorf("%d 未分類 projects, want 1", n)
	}
	// both clones made 未分類 the same, so the two updated times are one
	raw, err := os.ReadFile(filepath.Join(storeDir(t, b), filepath.FromSlash(model.TaskPath(unc)))) //nolint:gosec // the test's store
	if err != nil {
		t.Fatal(err)
	}
	if u, err := model.DecodeTask(raw); err != nil || !u.Updated.Equal(a.o.ws.Created) {
		t.Errorf("未分類 updated = %v, %v; want the creation of the workspace", u, err)
	}
	if e, ok := ix.Task(mine.ULID); !ok || e.Parent == nil || *e.Parent != unc {
		t.Errorf("its task at the root is not under 未分類: %+v", e)
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

// reverseRace makes the state of the race: a converts (with opt) and does not push, an
// edit of its own follows, and another clone pushes a task at the root and a file in format
// 1. d is a third clone that has nothing of its own; edited is the file a edited.
func reverseRace(t *testing.T, opt Options) (a *remoteFx, d *usecasetest.Session, theirs *model.Task, edited string) {
	t.Helper()
	a = newRemoteFx(t)
	d = cloneOld(t, a)
	_, e, err := a.ts.Entry()
	if err != nil {
		t.Fatal(err)
	}
	s, err := a.ts.OpenStoreAtAnyFormat(e.Path)
	if err != nil {
		t.Fatal(err)
	}
	// the conversion is committed here and not pushed
	if _, err := convertCommit(a.ts.Session, s, opt, 1, model.StoreFormat); err != nil {
		t.Fatal(err)
	}
	_ = s.Repo().Close()
	// a task of its own is edited after that (the project: the conversion leaves it as it is)
	edited = model.TaskPath(a.o.project.ULID)
	raw, err := os.ReadFile(filepath.Join(e.Path, filepath.FromSlash(edited))) //nolint:gosec // the test's store
	if err != nil {
		t.Fatal(err)
	}
	renamed := strings.Replace(string(raw), "title: web", "title: web renamed", 1)
	if renamed == string(raw) {
		t.Fatal("the title was not found in the file of the project")
	}
	commitRaw(t, a.ts, "edit after the conversion", map[string]string{edited: renamed})
	// the other clone pushes: a task at the root and a file, in format 1
	theirs, _ = rawTask(t, a.o.ws, 7, "task", "theirs at the root", nil)
	theirs.Rank, _ = model.Between("", "")
	enc, err := theirs.Encode()
	if err != nil {
		t.Fatal(err)
	}
	pushFile(t, a, model.TaskPath(theirs.ULID), string(enc))
	pushFile(t, a, "notes.txt", "theirs")
	a.ts.DropMount()
	return a, d, theirs, edited
}

// A clone converts, and before it pushes, another clone pushes a commit in the
// old format: the clone is in the newest format and origin in an older one, and
// the two are apart. hh store convert converts the changes of origin as a commit
// on origin's head, pushes it and merges.
func TestOriginOlderAndDivergedIsConvertedFromOrigin(t *testing.T) {
	t.Parallel()
	// the conversion of a is stamped with a time other than the one of the run that follows
	a, d, theirs, edited := reverseRace(t, Options{Now: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)})
	_, e, err := a.ts.Entry()
	if err != nil {
		t.Fatal(err)
	}
	// a sync says which way it goes
	c, _, err := a.ts.Entry()
	if err != nil {
		t.Fatal(err)
	}
	s, err := a.ts.OpenStoreAtAnyFormat(e.Path)
	if err != nil {
		t.Fatal(err)
	}
	opt, err := a.ts.SyncOptions(c, e)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.ts.RunSync(s, opt, true)
	_ = s.Repo().Close()
	var fd *replicate.FormatDivergedError
	if !errors.As(err, &fd) || !fd.RemoteOlder() {
		t.Fatalf("sync = %v, want a FormatDivergedError with the remote older", err)
	}
	wantCode(t, err, usecase.KindConflict, "sync.format_diverged")
	if ae := usecase.AsError(err); ae.Args["remote_older"] != true {
		t.Errorf("args = %v", ae.Args)
	}

	// a dry run lists what it would do and writes nothing
	before, remote := headOf(t, a.ts), originHead(t, a.remote)
	reports, err := Run(a.ts.Session, Options{}, true)
	if err != nil || len(reports) == 0 || !strings.Contains(strings.Join(reports[0].Lines, "\n"), "リモートの変更") {
		t.Fatalf("dry run = %+v, %v", reports, err)
	}
	if headOf(t, a.ts) != before || originHead(t, a.remote) != remote {
		t.Error("the dry run wrote")
	}

	reports, err = Run(a.ts.Session, Options{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) == 0 || reports[0].From != 1 || reports[0].To != model.StoreFormat {
		t.Errorf("reports = %+v", reports)
	}
	if formatOf(t, a.ts) != model.StoreFormat || originHead(t, a.remote) != headOf(t, a.ts) {
		t.Fatalf("format %d; origin is not at the head of the clone", formatOf(t, a.ts))
	}
	if b, err := os.ReadFile(filepath.Join(storeDir(t, a.ts), "notes.txt")); err != nil || string(b) != "theirs" { //nolint:gosec // the test's store
		t.Errorf("the change of the other clone: %q, %v", b, err)
	}
	// the edit a made after its conversion survived the merge
	if b, err := os.ReadFile(filepath.Join(storeDir(t, a.ts), filepath.FromSlash(edited))); err != nil || !strings.Contains(string(b), "title: web renamed") { //nolint:gosec // the test's store
		t.Errorf("the edit of its own task is gone: %q, %v", b, err)
	}
	// its task at the root was converted: under 未分類
	_, ix, err := a.ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	var unc model.ULID
	for _, e := range ix.Tasks() {
		if e.Title == merge.Uncategorized {
			unc = e.ULID
		}
	}
	if e, ok := ix.Task(theirs.ULID); !ok || unc.IsZero() || e.Parent == nil || *e.Parent != unc {
		t.Errorf("its task at the root is not under 未分類: %+v", e)
	}
	if r, err := a.ts.DoctorOne(false); err != nil {
		t.Fatal(err)
	} else {
		for _, f := range r.Findings {
			if f.Check != "merge_driver" {
				t.Errorf("finding after the way back: %+v", f)
			}
		}
	}
	// the third clone goes to origin by fast-forward, and a sync works from now on
	if _, err := Run(d.Session, Options{}, false); err != nil {
		t.Fatal(err)
	}
	if headOf(t, d) != originHead(t, a.remote) {
		t.Error("the third clone is not at the head of origin")
	}
	if reports, err := Run(a.ts.Session, Options{}, false); err != nil || len(reports) != 0 {
		t.Errorf("second run = %v, %v", reports, err)
	}
	if _, err := replication.Sync(a.ts.Session, false, true); err != nil {
		t.Errorf("sync after the way back: %v", err)
	}
}

// Another clone pushes while the changes of origin are converted: the push is refused, nothing
// is converted, the error is store.convert_unsynced, and running the command again works. Not
// parallel: it replaces beforeRemotePush.
func TestReverseConvertPushRefusedThenAgain(t *testing.T) {
	a, _, _, _ := reverseRace(t, Options{})
	pushed := false
	beforeRemotePush = func() {
		if !pushed {
			pushed = true
			pushFile(t, a, "more.txt", "again")
		}
	}
	t.Cleanup(func() { beforeRemotePush = func() {} })
	before := headOf(t, a.ts)
	_, err := Run(a.ts.Session, Options{}, false)
	wantCode(t, err, usecase.KindConflict, "store.convert_unsynced")
	if !strings.Contains(err.Error(), "run hh store convert again") {
		t.Errorf("the error does not say to run it again: %v", err)
	}
	if headOf(t, a.ts) != before {
		t.Error("the store changed")
	}
	reports, err := Run(a.ts.Session, Options{}, false)
	if err != nil || len(reports) == 0 {
		t.Fatalf("second run = %v, %v", reports, err)
	}
	if originHead(t, a.remote) != headOf(t, a.ts) || formatOf(t, a.ts) != model.StoreFormat {
		t.Error("origin is not at the head of the converted store")
	}
	for _, f := range []string{"notes.txt", "more.txt"} {
		if _, err := os.Stat(filepath.Join(storeDir(t, a.ts), f)); err != nil {
			t.Errorf("%s was lost: %v", f, err)
		}
	}
}

// The sync after the push can fail: the converted changes of origin are pushed already, the error
// tells to run hh sync, and hh sync finishes it.
func TestReverseConvertSyncFailsAfterThePush(t *testing.T) {
	t.Parallel()
	a, _, _, _ := reverseRace(t, Options{})
	// a file origin has and the store does not, written here without a commit: the merge would overwrite it
	if err := os.WriteFile(filepath.Join(storeDir(t, a.ts), "notes.txt"), []byte("mine, not committed"), 0o600); err != nil {
		t.Fatal(err)
	}
	reports, err := Run(a.ts.Session, Options{}, false)
	if err == nil || !strings.Contains(err.Error(), "run hh sync to finish") || !errors.Is(err, replicate.ErrDirty) {
		t.Fatalf("convert = %v, %v", reports, err)
	}
	if len(reports) == 0 {
		t.Error("the reports of what was pushed are not returned")
	}
	// the push happened: origin is in the newest format; converting again has nothing to do
	if reports, err := Run(a.ts.Session, Options{}, false); err != nil || len(reports) != 0 {
		t.Errorf("run again = %v, %v", reports, err)
	}
	// the cause is fixed, and hh sync finishes it
	if err := os.Remove(filepath.Join(storeDir(t, a.ts), "notes.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := replication.Sync(a.ts.Session, false, true); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if originHead(t, a.remote) != headOf(t, a.ts) || formatOf(t, a.ts) != model.StoreFormat {
		t.Error("origin is not at the head of the store after the sync")
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

func TestAnOriginOfANewerFormatIsNotUnsynced(t *testing.T) {
	t.Parallel()
	f := newRemoteFx(t)
	pushFile(t, f, model.StoreMetaPath, "ulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nname: me\nformat: 99\n")
	_, err := Run(f.ts.Session, Options{}, false)
	wantCode(t, err, usecase.KindInvalid, "store.newer_format")
}

// withFormat5 stands a fifth format in: the steps get one that writes
// step5.txt, and the newest format is 5. The test must not be parallel.
func withFormat5(t *testing.T) {
	t.Helper()
	was, wasSteps := model.StoreFormat, steps
	model.StoreFormat = 5
	steps = append(slices.Clone(steps), Step{From: 4, Apply: func(tr *Tree, _ Options) (Report, error) {
		tr.Write("step5.txt", []byte("x"))
		return Report{Lines: []string{"step5"}}, nil
	}})
	t.Cleanup(func() { model.StoreFormat, steps = was, wasSteps })
}

func TestAnOriginInAMiddleFormatIsFastForwardedToAndConvertedOn(t *testing.T) {
	a := newRemoteFx(t)
	b := cloneOld(t, a)
	if _, err := Run(a.ts.Session, Options{}, false); err != nil { // origin is in format 4
		t.Fatal(err)
	}
	converted := originHead(t, a.remote)
	withFormat5(t)
	reports, err := Run(b.Session, Options{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 2 || reports[0].From != 1 || reports[0].To != 4 || reports[1].From != 4 || reports[1].To != 5 {
		t.Fatalf("reports = %+v", reports)
	}
	if !strings.Contains(strings.Join(reports[0].Lines, "\n"), "fast-forward") {
		t.Errorf("the first report is not the fast-forward: %v", reports[0].Lines)
	}
	if formatOf(t, b) != 5 || headOf(t, b) != originHead(t, a.remote) {
		t.Errorf("format %d, head %s, origin %s", formatOf(t, b), headOf(t, b), originHead(t, a.remote))
	}
	if _, err := os.Stat(filepath.Join(storeDir(t, b), "step5.txt")); err != nil {
		t.Error(err)
	}
	// one commit of its own, on the conversion of the first clone
	r, err := gitrepo.Open(storeDir(t, b))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	h, _ := r.Head()
	log, err := r.LogFrom(h, 2)
	if err != nil || len(log) != 2 || log[1].Hash.String() != converted {
		t.Errorf("log = %v, %v; want the conversion of step 5 on %s", log, err, converted)
	}
}

func TestAnOriginInAMiddleFormatWithChangesNotPushed(t *testing.T) {
	a := newRemoteFx(t)
	b := cloneOld(t, a)
	commitRaw(t, b, "mine", map[string]string{"notes.txt": "mine"})
	if _, err := Run(a.ts.Session, Options{}, false); err != nil { // origin is in format 4
		t.Fatal(err)
	}
	withFormat5(t)
	reports, err := Run(b.Session, Options{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 4 || reports[0].To != 2 || reports[1].To != 3 || reports[2].To != 4 || reports[3].From != 4 || reports[3].To != 5 {
		t.Fatalf("reports = %+v", reports)
	}
	if formatOf(t, b) != 5 || headOf(t, b) != originHead(t, a.remote) {
		t.Errorf("format %d; the clone is not what origin has", formatOf(t, b))
	}
	for _, f := range []string{"step5.txt", "notes.txt"} {
		if _, err := os.Stat(filepath.Join(storeDir(t, b), f)); err != nil {
			t.Error(err)
		}
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
