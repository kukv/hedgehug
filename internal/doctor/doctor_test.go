package doctor

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"

	"github.com/kukv/hedgehug/internal/model"
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
	c := &model.Comment{ID: model.NewID(), Author: "x", Created: now, Updated: now, Body: "hi"}
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
	rel := model.CommentPath(task.ID, c.ID)
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
	writeFile(t, dir, model.StoreMetaPath, "id: [\n")
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
	// b's content at a's path: the ID and the path disagree.
	bb, err := b.Encode()
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, model.TaskPath(a.ID), string(bb))
	writeFile(t, dir, "tasks/zz/zz/nope.md", "x")
	undecodable := model.TaskPath(model.NewID())
	writeFile(t, dir, undecodable, "not a task")
	orphan := model.NewID()
	c, cb := newComment(t)
	writeFile(t, dir, model.CommentPath(orphan, c.ID), cb)
	writeFile(t, dir, "templates/.bad.md", "x")
	writeFile(t, dir, "sprints/nope.md", "x")
	fs := run(t, dir)

	if f := find(t, fs, "file", model.TaskPath(a.ID)); !strings.Contains(f.Msg, b.ID.String()) {
		t.Errorf("mismatch = %+v", f)
	}
	if f := find(t, fs, "file", "tasks/zz/zz/nope.md"); !strings.Contains(f.Msg, "name") {
		t.Errorf("name = %+v", f)
	}
	if f := find(t, fs, "file", undecodable); !strings.Contains(f.Msg, "decode") {
		t.Errorf("decode = %+v", f)
	}
	if f := find(t, fs, "file", model.CommentPath(orphan, c.ID)); !strings.Contains(f.Msg, "no task file") {
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
	missing := model.NewID()
	a.Parent = &missing
	a.Key = "TST-1"
	putRaw(t, s, dir, a)
	fs := run(t, dir)
	if len(fs) != 1 || fs[0].Check != "task" || fs[0].Where != "TST-1 ("+a.ID.String()+") A" || !strings.Contains(fs[0].Msg, "parent") {
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
	a.Parent, b.Parent = &b.ID, &a.ID
	putRaw(t, s, dir, a)
	putRaw(t, s, dir, b)
	rel := model.TaskPath(model.NewID())
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

func TestRunWorkspaces(t *testing.T) {
	s, dir := newStore(t)
	w := &model.Workspace{ID: model.NewID(), Name: "other", KeyPrefix: "TST"}
	b, err := w.Encode()
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, model.WorkspacePath(w.ID), string(b))
	if _, err := s.Repo().Commit(commitOf(model.WorkspacePath(w.ID))); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, f := range run(t, dir) {
		if f.Check == "workspace" && strings.Contains(f.Msg, "key prefix TST") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d workspace findings, want 1", n)
	}
	writeFile(t, dir, "workspaces/nope.md", "x")
	find(t, run(t, dir), "file", "workspaces/nope.md")
}

func TestRunWorkspaceFields(t *testing.T) {
	_, dir := newStore(t)
	set := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		w    model.Workspace
		want string
	}{
		{model.Workspace{Name: "a", KeyPrefix: "me", PrefixSet: set}, "key prefix"},
		{model.Workspace{Name: "B", KeyPrefix: "BB", PrefixSet: set}, "cannot name a workspace"},
		{model.Workspace{KeyPrefix: "CC", PrefixSet: set}, "name is missing"},
		{model.Workspace{Name: "d", PrefixSet: set}, "key_prefix is missing"},
		{model.Workspace{Name: "e", KeyPrefix: "EE"}, "prefix_set is missing"},
	} {
		tc.w.ID = model.NewID()
		b, err := tc.w.Encode()
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, dir, model.WorkspacePath(tc.w.ID), string(b))
		if f := find(t, run(t, dir), "file", model.WorkspacePath(tc.w.ID)); !strings.Contains(f.Msg, tc.want) {
			t.Errorf("finding = %+v, want %q", f, tc.want)
		}
	}
}

func TestRunNumbers(t *testing.T) {
	s, dir := newStore(t)
	for _, tt := range []string{"TST: [\n", "t: 3\n", "TST: 0\n"} {
		writeFile(t, dir, model.NumbersPath, tt)
		if _, err := s.Repo().Commit(commitOf(model.NumbersPath)); err != nil {
			t.Fatal(err)
		}
		find(t, run(t, dir), "file", model.NumbersPath)
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

func TestRunKeyProblems(t *testing.T) {
	s, dir := newStore(t)
	a := newTask("A")
	a.Key = "XX-1"
	putRaw(t, s, dir, a)
	fs := run(t, dir)
	if f := find(t, fs, "task", "XX-1 ("+a.ID.String()+") A"); !strings.Contains(f.Msg, "key prefix") {
		t.Errorf("finding = %+v", f)
	}
}
