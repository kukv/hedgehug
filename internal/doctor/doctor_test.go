package doctor

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
	"github.com/kukv/hedgehug/internal/view"
)

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, dir string) []Finding {
	t.Helper()
	fs, err := Run(dir)
	if err != nil {
		t.Fatal(err)
	}
	return fs
}

// find returns the finding of check that is at where, or fails.
func find(t *testing.T, fs []Finding, check, where string) Finding {
	t.Helper()
	for _, f := range fs {
		if f.Check == check && f.Where == where {
			return f
		}
	}
	t.Fatalf("no %s finding at %q in %+v", check, where, fs)
	return Finding{}
}

func newComment(t *testing.T) (*model.Comment, string) {
	t.Helper()
	now := time.Now().UTC()
	c := &model.Comment{ULID: model.NewULID(), Author: "x", Created: now, Updated: now, Body: "hi"}
	b, err := c.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return c, string(b)
}

func TestRunClean(t *testing.T) {
	s, dir := newStore(t)
	task := newTask("A")
	put(t, s, task)
	c, body := newComment(t)
	rel := model.CommentPath(task.ULID, c.ULID)
	writeFile(t, dir, rel, body)
	if _, err := s.Repo().Commit(commitOf(rel)); err != nil {
		t.Fatal(err)
	}
	if fs := run(t, dir); len(fs) != 0 {
		t.Errorf("findings = %+v, want none", fs)
	}
}

func TestRunBrokenSchema(t *testing.T) {
	s, dir := newStore(t)
	put(t, s, newTask("A"))
	writeFile(t, dir, "schema.yaml", "statuses: [\n")
	fs := run(t, dir)
	var schemaFs []Finding
	for _, f := range fs {
		if f.Check == "schema" {
			schemaFs = append(schemaFs, f)
		}
	}
	if len(schemaFs) != 2 || !strings.Contains(schemaFs[0].Hint, "git restore schema.yaml") || !strings.Contains(schemaFs[1].Msg, "skipped") {
		t.Fatalf("schema findings = %+v", schemaFs)
	}
	f := find(t, fs, "worktree", "schema.yaml")
	if f.Hint != "git restore schema.yaml, or remove the file if it is new" {
		t.Errorf("hint = %q", f.Hint)
	}
	for _, f := range fs {
		if f.Check == "task" || f.Check == "git" {
			t.Errorf("step 5 ran: %+v", f)
		}
	}
}

func TestRunBrokenStoreMeta(t *testing.T) {
	_, dir := newStore(t)
	writeFile(t, dir, model.StoreMetaPath, "ulid: [\n")
	fs := run(t, dir)
	find(t, fs, "file", model.StoreMetaPath)
	for _, f := range fs {
		if f.Check == "task" || f.Check == "git" {
			t.Errorf("step 5 ran: %+v", f)
		}
	}
}

func TestRunFiles(t *testing.T) {
	s, dir := newStore(t)
	a, b := newTask("A"), newTask("B")
	put(t, s, a)
	// b's content at a's path: the ULID and the path disagree.
	bb, err := b.Encode()
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, model.TaskPath(a.ULID), string(bb))
	writeFile(t, dir, "tasks/zz/zz/nope.md", "x")
	undecodable := model.TaskPath(model.NewULID())
	writeFile(t, dir, undecodable, "not a task")
	orphan := model.NewULID()
	c, cb := newComment(t)
	writeFile(t, dir, model.CommentPath(orphan, c.ULID), cb)
	writeFile(t, dir, "templates/.bad.md", "x")
	writeFile(t, dir, "sprints/nope.md", "x")
	fs := run(t, dir)

	if f := find(t, fs, "file", model.TaskPath(a.ULID)); !strings.Contains(f.Msg, b.ULID.String()) {
		t.Errorf("mismatch = %+v", f)
	}
	if f := find(t, fs, "file", "tasks/zz/zz/nope.md"); !strings.Contains(f.Msg, "name") {
		t.Errorf("name = %+v", f)
	}
	if f := find(t, fs, "file", undecodable); !strings.Contains(f.Msg, "decode") {
		t.Errorf("decode = %+v", f)
	}
	if f := find(t, fs, "file", model.CommentPath(orphan, c.ULID)); !strings.Contains(f.Msg, "no task file") {
		t.Errorf("orphan = %+v", f)
	}
	find(t, fs, "file", "templates/.bad.md")
	find(t, fs, "file", "sprints/nope.md")
}

func TestRunWorktree(t *testing.T) {
	s, dir := newStore(t)
	put(t, s, newTask("A"))
	writeFile(t, dir, "untracked.md", "x")
	writeFile(t, dir, "staged.md", "x")
	r, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	w, err := r.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Add("staged.md"); err != nil {
		t.Fatal(err)
	}
	fs := run(t, dir)
	u := find(t, fs, "worktree", "untracked.md")
	if u.Hint != "git restore untracked.md, or remove the file if it is new" {
		t.Errorf("unstaged = %+v", u)
	}
	st := find(t, fs, "worktree", "staged.md")
	if st.Hint != "git restore --staged staged.md" || !strings.Contains(st.Msg, "next") {
		t.Errorf("staged = %+v", st)
	}
}

func TestRunVerifyFails(t *testing.T) {
	s, dir := newStore(t)
	put(t, s, newTask("A"))
	matches, err := filepath.Glob(filepath.Join(dir, ".git", "objects", "??", "*"))
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range matches {
		if err := os.Chmod(m, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Truncate(m, 5); err != nil {
			t.Fatal(err)
		}
	}
	fs := run(t, dir)
	// Status fails on the same objects, so there may be two git findings.
	if !slices.ContainsFunc(fs, func(f Finding) bool {
		return f.Check == "git" && strings.HasPrefix(f.Msg, "a git object cannot be read")
	}) {
		t.Errorf("findings = %+v, want the Verify finding", fs)
	}
}

func TestRunIndexProblems(t *testing.T) {
	s, dir := newStore(t)
	a := newTask("A")
	missing := model.NewULID()
	a.Parent = &missing
	a.ID = "TST-1"
	putRaw(t, s, dir, a)
	fs := run(t, dir)
	if len(fs) != 1 || fs[0].Check != "task" || fs[0].Where != "TST-1 ("+a.ULID.String()+") A" || !strings.Contains(fs[0].Msg, "parent") {
		t.Errorf("findings = %+v", fs)
	}
}

func TestRunSchemaProblems(t *testing.T) {
	s, dir := newStore(t)
	bad := newTask("Bad")
	bad.Status = "weird"
	putRaw(t, s, dir, bad)
	fs := run(t, dir)
	if got := checks(fs); !slices.Equal(got, []string{"task"}) {
		t.Errorf("checks = %v\n%+v", got, fs)
	}
}

func TestRunBrokenHead(t *testing.T) {
	s, dir := newStore(t)
	put(t, s, newTask("A"))
	// Commit a broken schema.yaml, then make the file readable again.
	good, err := os.ReadFile(filepath.Join(dir, "schema.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "schema.yaml", "statuses: [\n")
	if _, err := s.Repo().Commit(commitOf("schema.yaml")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "schema.yaml", string(good))
	fs := run(t, dir)
	g := slices.IndexFunc(fs, func(f Finding) bool { return f.Check == "git" })
	if g < 0 {
		t.Fatalf("findings = %+v, want a git finding", fs)
	}
}

func TestRunCycleAndBroken(t *testing.T) {
	s, dir := newStore(t)
	a, b := newTask("A"), newTask("B")
	a.Parent, b.Parent = &b.ULID, &a.ULID
	putRaw(t, s, dir, a)
	putRaw(t, s, dir, b)
	rel := model.TaskPath(model.NewULID())
	writeFile(t, dir, rel, "not a task")
	if _, err := s.Repo().Commit(commitOf(rel)); err != nil {
		t.Fatal(err)
	}
	fs := run(t, dir)
	var cycles, others int
	for _, f := range fs {
		if f.Check != "task" {
			continue
		}
		if strings.Contains(f.Msg, "cycle") {
			cycles++
		} else {
			others++
		}
	}
	if cycles != 2 || others != 1 {
		t.Errorf("cycles = %d, others = %d, want 2 and 1\n%+v", cycles, others, fs)
	}
}

func TestRunIgnoresLockAndGitignore(t *testing.T) {
	s, dir := newStore(t)
	put(t, s, newTask("A"))
	if _, err := os.Stat(filepath.Join(dir, ".hedgehug", "local", "lock")); err != nil {
		t.Fatalf("the lock file is missing: %v", err)
	}
	if fs := run(t, dir); len(fs) != 0 {
		t.Errorf("findings = %+v, want none", fs)
	}
}

// commitWorkspace writes w into the store and commits it.
func commitWorkspace(t *testing.T, s *store.Store, dir string, w *model.Workspace) {
	t.Helper()
	b, err := w.Encode()
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, model.WorkspacePath(w.ULID), string(b))
	if _, err := s.Repo().Commit(commitOf(model.WorkspacePath(w.ULID))); err != nil {
		t.Fatal(err)
	}
}

// duplicateFindings returns, by where they are, the workspace findings that
// say what.
func duplicateFindings(t *testing.T, dir, what string) map[string]Finding {
	t.Helper()
	out := map[string]Finding{}
	for _, f := range run(t, dir) {
		if f.Check == "workspace" && strings.Contains(f.Msg, what) {
			out[f.Where] = f
		}
	}
	return out
}

func TestRunWorkspaces(t *testing.T) {
	s, dir := newStore(t)
	first := firstWorkspace(t, s)
	w := &model.Workspace{ULID: model.NewULID(), Name: "other", IDPrefix: "TST"}
	commitWorkspace(t, s, dir, w)
	got := duplicateFindings(t, dir, "id prefix TST")
	// Both are reported, each naming the other.
	if len(got) != 2 || !strings.Contains(got[model.WorkspacePath(first)].Msg, w.ULID.String()) || !strings.Contains(got[model.WorkspacePath(w.ULID)].Msg, first.String()) {
		t.Errorf("findings = %+v", got)
	}
	writeFile(t, dir, "workspaces/nope.md", "x")
	find(t, run(t, dir), "file", "workspaces/nope.md")
}

func TestRunWorkspaceNames(t *testing.T) {
	s, dir := newStore(t)
	first := firstWorkspace(t, s)
	w := &model.Workspace{ULID: model.NewULID(), Name: "test", IDPrefix: "OTHER"} // as the first one
	commitWorkspace(t, s, dir, w)
	got := duplicateFindings(t, dir, "name test")
	if len(got) != 2 || !strings.Contains(got[model.WorkspacePath(first)].Msg, w.ULID.String()) || !strings.Contains(got[model.WorkspacePath(w.ULID)].Msg, first.String()) {
		t.Errorf("findings = %+v", got)
	}
}

func TestRunWorkspaceFields(t *testing.T) {
	_, dir := newStore(t)
	set := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		w    model.Workspace
		want string
	}{
		{model.Workspace{Name: "a", IDPrefix: "me", PrefixSet: set}, "id prefix"},
		{model.Workspace{Name: "B", IDPrefix: "BB", PrefixSet: set}, "cannot name a workspace"},
		{model.Workspace{IDPrefix: "CC", PrefixSet: set}, "name is missing"},
		{model.Workspace{Name: "d", PrefixSet: set}, "id_prefix is missing"},
		{model.Workspace{Name: "e", IDPrefix: "EE"}, "prefix_set is missing"},
	} {
		tc.w.ULID = model.NewULID()
		b, err := tc.w.Encode()
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, dir, model.WorkspacePath(tc.w.ULID), string(b))
		if f := find(t, run(t, dir), "file", model.WorkspacePath(tc.w.ULID)); !strings.Contains(f.Msg, tc.want) {
			t.Errorf("finding = %+v, want %q", f, tc.want)
		}
	}
}

func TestRunNumbers(t *testing.T) {
	s, dir := newStore(t)
	for _, tt := range []struct{ data, want string }{
		{"TST: [\n", "does not decode: "},
		{"t: 3\n", "has an invalid entry: t: "},
		{"TST: 0\n", "has an invalid entry: TST: the number 0 is less than 1"},
	} {
		writeFile(t, dir, model.NumbersPath, tt.data)
		if _, err := s.Repo().Commit(commitOf(model.NumbersPath)); err != nil {
			t.Fatal(err)
		}
		if f := find(t, run(t, dir), "file", model.NumbersPath); !strings.HasPrefix(f.Msg, tt.want) {
			t.Errorf("%q: msg = %q, want it to start with %q", tt.data, f.Msg, tt.want)
		}
	}
	writeFile(t, dir, model.NumbersPath, "TST: 3\n")
	if _, err := s.Repo().Commit(commitOf(model.NumbersPath)); err != nil {
		t.Fatal(err)
	}
	for _, f := range run(t, dir) {
		if f.Where == model.NumbersPath {
			t.Errorf("good numbers.yaml reported: %+v", f)
		}
	}
}

func TestRunIDProblems(t *testing.T) {
	s, dir := newStore(t)
	a := newTask("A")
	a.ID = "XX-1"
	putRaw(t, s, dir, a)
	fs := run(t, dir)
	if f := find(t, fs, "task", "XX-1 ("+a.ULID.String()+") A"); !strings.Contains(f.Msg, "id prefix") {
		t.Errorf("finding = %+v", f)
	}
}

// hasFinding reports whether fs has a finding of check whose message or
// hint contains text.
func hasFinding(fs []Finding, check, text string) bool {
	return slices.ContainsFunc(fs, func(f Finding) bool {
		return f.Check == check && (strings.Contains(f.Msg, text) || strings.Contains(f.Hint, text))
	})
}

// registerDriver writes the merge driver of .git/config by hand, with
// driver as its command.
func registerDriver(t *testing.T, dir, driver string) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(dir, ".git", "config"), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString("[merge \"hedgehug\"]\n\tname = Hedgehug field merge\n\tdriver = " + driver + "\n")
	if err = errors.Join(err, f.Close()); err != nil {
		t.Fatal(err)
	}
}

func TestMergeDriverChecks(t *testing.T) {
	dir := newBareStore(t) // .gitattributes with merge=hedgehug, no driver in .git/config
	if fs := run(t, dir); !hasFinding(fs, "merge_driver", "not registered") || !hasFinding(fs, "merge_driver", "run hedgehug sync once") {
		t.Errorf("findings = %+v", fs)
	}
	registerDriver(t, dir, "hedgehug merge-driver %O %A %B")
	if fs := run(t, dir); !hasFinding(fs, "merge_driver", "not registered") {
		t.Errorf("a different command: %+v", fs)
	}

	dir = newBareStore(t)
	registerDriver(t, dir, gitrepo.MergeDriver)
	lookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	if fs := run(t, dir); !hasFinding(fs, "merge_driver", "hedgehug is not on PATH") {
		t.Errorf("findings = %+v", fs)
	}
	lookPath = func(string) (string, error) { return "/bin/hedgehug", nil }
	for _, f := range run(t, dir) {
		if f.Check == "merge_driver" {
			t.Errorf("registered and on PATH: %+v", f)
		}
	}
}

func TestMergeDriverIgnoredWithoutAttributes(t *testing.T) {
	dir := newBareStore(t)
	if err := os.Remove(filepath.Join(dir, store.AttributesPath)); err != nil {
		t.Fatal(err)
	}
	for _, f := range run(t, dir) {
		if f.Check == "merge_driver" {
			t.Errorf("no merge=hedgehug line: %+v", f)
		}
	}
}

func TestUnimportedRecords(t *testing.T) {
	_, dir := newStore(t)
	rec := `{"path":"tasks/x.md","field":"title","value":"\"a\"","author":"a","commit":"","time":"2026-10-31T00:00:00Z"}`
	writeFile(t, dir, ".hedgehug/local/overwritten/01JC1111111111111111111111.json", rec)
	writeFile(t, dir, ".hedgehug/local/overwritten/01JC2222222222222222222222.json", rec)
	fs := run(t, dir)
	if !hasFinding(fs, "inbox", "2 merge driver records are not in the inbox yet") || !hasFinding(fs, "inbox", "hedgehug inbox") {
		t.Errorf("findings = %+v", fs)
	}
}

func TestUndecodableRecords(t *testing.T) {
	_, dir := newStore(t)
	writeFile(t, dir, ".hedgehug/local/overwritten/01JC1111111111111111111111.json", "{}")
	writeFile(t, dir, ".hedgehug/local/overwritten/01JC2222222222222222222222.json", "[")
	writeFile(t, dir, ".hedgehug/local/overwritten/note.txt", "not a record")
	fs := run(t, dir)
	for _, name := range []string{"01JC1111111111111111111111", "01JC2222222222222222222222"} {
		f := find(t, fs, "inbox", ".hedgehug/local/overwritten/"+name+".json")
		if !strings.Contains(f.Msg, "does not decode") || !strings.Contains(f.Hint, "remove the file") {
			t.Errorf("%s: %+v", name, f)
		}
	}
	if hasFinding(fs, "inbox", "hedgehug inbox") || len(fs) != 2 {
		t.Errorf("an undecodable record is not waiting for the inbox: %+v", fs)
	}
}

func TestNoRecords(t *testing.T) {
	_, dir := newStore(t)
	if fs := run(t, dir); len(fs) != 0 {
		t.Errorf("findings = %+v", fs)
	}
}

func TestFixHints(t *testing.T) {
	s, dir := newStore(t)
	a, b := newTask("a"), newTask("b")
	a.ID, b.ID = "TST-7", "TST-7"
	putRaw(t, s, dir, a)
	putRaw(t, s, dir, b)
	fs := run(t, dir)
	for _, f := range fs {
		if f.Check == "task" && strings.Contains(f.Msg, "TST-7") {
			if !strings.Contains(f.Hint, "hedgehug doctor --fix") || !strings.Contains(f.Hint, "hedgehug sync") {
				t.Errorf("hint = %q", f.Hint)
			}
			return
		}
	}
	t.Errorf("no duplicate_id finding in %+v", fs)
}

func TestFix(t *testing.T) {
	s, dir := newStore(t)
	a, b := newTask("a"), newTask("b")
	a.ID, b.ID = "TST-7", "TST-7"
	putRaw(t, s, dir, a)
	putRaw(t, s, dir, b)
	opt := sync.Options{Author: testAuthor, Now: time.Now}
	repaired, err := Fix(s, opt)
	if err != nil {
		t.Fatal(err)
	}
	if len(repaired) != 1 || repaired[0].Kind != "id_collision" {
		t.Fatalf("repaired = %+v", repaired)
	}
	if fs := run(t, dir); len(fs) != 0 {
		t.Errorf("after fix: %+v", fs)
	}
	if again, err := Fix(s, opt); err != nil || len(again) != 0 {
		t.Errorf("second fix = %+v, %v", again, err)
	}
}

func TestFixHintsByProblem(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, s *store.Store, dir string)
		msg   string
	}{
		{"id_prefix", func(t *testing.T, s *store.Store, dir string) {
			a := newTask("a")
			a.ID = "XX-1"
			putRaw(t, s, dir, a)
		}, "id prefix"},
		{"cycle", func(t *testing.T, s *store.Store, dir string) {
			a, b := newTask("a"), newTask("b")
			a.Parent, b.Parent = &b.ULID, &a.ULID
			putRaw(t, s, dir, a)
			putRaw(t, s, dir, b)
		}, "cycle"},
		{"missing parent", func(t *testing.T, s *store.Store, dir string) {
			a := newTask("a")
			gone := model.NewULID()
			a.Parent = &gone
			putRaw(t, s, dir, a)
		}, "not in this store"},
		{"deleted parent", func(t *testing.T, s *store.Store, dir string) {
			p, c := newTask("p"), newTask("c")
			now := time.Now()
			p.DeletedAt = &now
			c.Parent = &p.ULID
			putRaw(t, s, dir, p)
			putRaw(t, s, dir, c)
		}, "is deleted"},
		{"workspace_mismatch", func(t *testing.T, s *store.Store, dir string) {
			w := &model.Workspace{ULID: model.NewULID(), Name: "two", IDPrefix: "TWO", PrefixSet: time.Now()}
			commitWorkspace(t, s, dir, w)
			p, c := newTask("p"), newTask("c")
			p.Workspace, p.ID = w.ULID, "TWO-1"
			c.Parent = &p.ULID
			putRaw(t, s, dir, p)
			putRaw(t, s, dir, c)
		}, "workspace of the parent"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, dir := newStore(t)
			tt.setup(t, s, dir)
			for _, f := range run(t, dir) {
				if f.Check == "task" && strings.Contains(f.Msg, tt.msg) {
					if !strings.Contains(f.Hint, "hedgehug doctor --fix") {
						t.Errorf("hint = %q", f.Hint)
					}
					return
				}
			}
			t.Errorf("no finding with %q", tt.msg)
		})
	}
}

func TestLiveChildOfDeletedParentIsFixed(t *testing.T) {
	s, dir := newStore(t)
	p, c := newTask("p"), newTask("c")
	now := time.Now()
	p.DeletedAt = &now
	c.Parent = &p.ULID
	putRaw(t, s, dir, p)
	putRaw(t, s, dir, c)
	repaired, err := Fix(s, sync.Options{Author: testAuthor, Now: time.Now})
	if err != nil || len(repaired) != 1 || repaired[0].Kind != "parent_deleted" {
		t.Fatalf("repaired = %+v, %v", repaired, err)
	}
	if fs := run(t, dir); len(fs) != 0 {
		t.Errorf("after fix: %+v", fs)
	}
}

func TestDoctorViewColumnsWithBrokenStoreMeta(t *testing.T) {
	s, dir := newStore(t)
	if _, err := s.Update(store.Op{Name: "test", Author: testAuthor}, func(tx *store.Tx) error {
		return tx.PutFile("views/cols.yaml", []byte("layout: table\ntable:\n  columns: [id, gone]\n"))
	}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, model.StoreMetaPath, "ulid: [\n")
	fs := run(t, dir)
	find(t, fs, "file", model.StoreMetaPath)
	find(t, fs, "view", "views/cols.yaml")
}

func TestDoctorViews(t *testing.T) {
	s, dir := newStore(t)
	files := map[string]string{
		"views/ok.yaml":     "layout: table\ntable:\n  columns: [id, due]\n",
		"views/cols.yaml":   "layout: table\ntable:\n  columns: [id, gone]\n",
		"views/bad.yaml":    "layout: gantt\n",
		"views/broken.yaml": "- x\n",
		"views/multi.yaml":  "layout: gantt\ntable:\n  group_by: owner\n",
		"views/trail .yaml": "layout: tree\n",
		"views/Same.yaml":   "layout: tree\n",
		"views/same.yaml":   "layout: tree\n",
		"views/sub/x.yaml":  "layout: tree\n",
		"views/notes.txt":   "x",
	}
	want := map[string][]string{
		"view views/cols.yaml":   {`column "gone" is not a field or a property of the schema; the GUI skips it`},
		"file views/bad.yaml":    {`layout "gantt" is not one of tree, kanban, table`},
		"file views/broken.yaml": nil, // checked by prefix below
		"file views/multi.yaml":  nil, // checked in order below
		"file views/trail .yaml": nil,
		"file views/Same.yaml":   {"has the same name as views/same.yaml but for case; rename one of them with hedgehug view rename"},
		"file views/same.yaml":   {"has the same name as views/Same.yaml but for case; rename one of them with hedgehug view rename"},
		"file views/sub/x.yaml":  {"is not a view: views holds <name>.yaml files only"},
		"file views/notes.txt":   {"is not a view: views holds <name>.yaml files only"},
	}
	if !caseSensitive(t, dir) {
		for _, k := range []string{"Same", "same"} {
			delete(files, "views/"+k+".yaml")
			delete(want, "file views/"+k+".yaml")
		}
	}
	if runtime.GOOS == "windows" {
		delete(files, "views/trail .yaml")
		delete(want, "file views/trail .yaml")
	}
	if _, err := s.Update(store.Op{Name: "test", Author: testAuthor}, func(tx *store.Tx) error {
		for rel, data := range files {
			if err := tx.PutFile(rel, []byte(data)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	fs, err := Run(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, f := range fs {
		if strings.HasPrefix(f.Where, "views/") {
			got[f.Check+" "+f.Where] = append(got[f.Check+" "+f.Where], f.Msg)
		}
	}
	if len(got) != len(want) {
		t.Errorf("findings = %v", got)
	}
	for k, w := range want {
		g, ok := got[k]
		switch {
		case !ok:
			t.Errorf("no finding %s (all: %v)", k, got)
		case k == "file views/broken.yaml":
			if len(g) != 1 || !strings.HasPrefix(g[0], "does not decode: ") {
				t.Errorf("%s = %q", k, g)
			}
		case k == "file views/multi.yaml":
			v, derr := view.Decode("multi", []byte(files["views/multi.yaml"]))
			if derr != nil {
				t.Fatal(derr)
			}
			if ps := v.Problems(); len(ps) < 2 || !slices.Equal(g, ps) {
				t.Errorf("%s = %q; want each of %q once, in order", k, g, ps)
			}
		case k == "file views/trail .yaml":
			if len(g) != 1 || !strings.HasPrefix(g[0], "the name is not a valid view name: ") {
				t.Errorf("%s = %q", k, g)
			}
		case !slices.Equal(g, w):
			t.Errorf("%s = %q; want %q", k, g, w)
		}
	}
}

// caseSensitive reports whether the file system of dir tells names apart by
// case. Windows and macOS do not, and cannot hold Same.yaml and same.yaml.
func caseSensitive(t *testing.T, dir string) bool {
	t.Helper()
	p := filepath.Join(dir, ".hedgehug", "local", "caseprobe")
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := os.Stat(filepath.Join(filepath.Dir(p), "CASEPROBE"))
	return err != nil
}
