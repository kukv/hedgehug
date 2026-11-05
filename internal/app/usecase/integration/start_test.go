package integration_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/integration"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/gitrepo/gitrepotest"
)

// codeRepo makes a work tree with one commit on main and returns its directory.
func codeRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	_, err := git.PlainInitWithOptions(dir, &git.PlainInitOptions{
		InitOptions: git.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName("main")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitrepotest.CommitFiles(t, dir, "first", "t", "t@example.com", time.Unix(1_700_000_000, 0), "a.txt")
	return dir
}

func branchAt(t *testing.T, dir string) string {
	t.Helper()
	r, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := r.Reference(plumbing.HEAD, false)
	if err != nil {
		t.Fatal(err)
	}
	return string(ref.Target())
}

// addBranch makes a branch at HEAD and stays where HEAD is.
func addBranch(t *testing.T, dir, name string) {
	t.Helper()
	r, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	head, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName(name), head.Hash())); err != nil {
		t.Fatal(err)
	}
}

// switchTo moves HEAD to the branch name, and leaves the files.
func switchTo(t *testing.T, dir, name string) {
	t.Helper()
	r, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName(name))); err != nil {
		t.Fatal(err)
	}
}

// detach points HEAD at the commit it is on.
func detach(t *testing.T, dir string) {
	t.Helper()
	r, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	head, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Storer.SetReference(plumbing.NewHashReference(plumbing.HEAD, head.Hash())); err != nil {
		t.Fatal(err)
	}
}

func hasBranch(t *testing.T, dir, name string) bool {
	t.Helper()
	r, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Reference(plumbing.NewBranchReferenceName(name), false)
	return err == nil
}

// taskOf is the task ME-1 of the store ss works on.
func taskOf(t *testing.T, ss *usecase.Session) *model.Task {
	t.Helper()
	s, ix, err := ss.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, err := ss.Resolve(ix, "ME-1")
	if err != nil {
		t.Fatal(err)
	}
	x, err := s.Task(e.ULID)
	if err != nil {
		t.Fatal(err)
	}
	return x
}

func TestSlug(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ title, want string }{
		{"Fix the login", "fix-the-login"},
		{"  Hello, World!  ", "hello-world"},
		{"a--b__c", "a-b-c"},
		{"ログインを直す", ""},
		{"ログイン fix 2", "fix-2"},
		{"", ""},
		{"1234567890123456789012345678901234567890", "1234567890123456789012345678901234567890"},
		{"12345678901234567890123456789012345678901", "1234567890123456789012345678901234567890"},
		// Cut in a word, and a hyphen at the cut goes.
		{"123456789012345678901234567890123456789 ab", "123456789012345678901234567890123456789"},
		{"1234567890123456789012345678901234567 abcdef", "1234567890123456789012345678901234567-ab"},
	} {
		if got := integration.Slug(c.title); got != c.want {
			t.Errorf("Slug(%q) = %q, want %q", c.title, got, c.want)
		}
	}
}

func TestStartMakesTheBranchAndTakesTheTask(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("Fix the login", nil)
	dir := codeRepo(t)
	r, err := integration.Start(ts.Session, "ME-1", dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Branch != "hh/ME-1-fix-the-login" || !r.Created || r.Switch || !r.Committed {
		t.Fatalf("result = %+v", r)
	}
	if got := branchAt(t, dir); got != "refs/heads/hh/ME-1-fix-the-login" {
		t.Errorf("HEAD = %s", got)
	}
	if op := usecasetest.LastOp(t, ts); op != "start" {
		t.Errorf("op = %s", op)
	}
	me, err := ts.AccountID()
	if err != nil {
		t.Fatal(err)
	}
	x := taskOf(t, ts.Session)
	if x.Status != "in_progress" || !slices.Equal(x.Assignees, []string{me}) || !slices.Contains(x.Watchers, me) {
		t.Errorf("task = status %s, assignees %v, watchers %v", x.Status, x.Assignees, x.Watchers)
	}
}

func TestStartOfATitleWithNoSlug(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("ログインを直す", nil)
	r, err := integration.Start(ts.Session, "ME-1", codeRepo(t))
	if err != nil || r.Branch != "hh/ME-1" {
		t.Fatalf("result = %+v, %v", r, err)
	}
}

func TestStartAgainChangesNothing(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("a", nil)
	dir := codeRepo(t)
	if _, err := integration.Start(ts.Session, "ME-1", dir); err != nil {
		t.Fatal(err)
	}
	head := ts.Head()
	r, err := integration.Start(ts.Session, "ME-1", dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Committed || r.Created || r.Switch || r.Branch != "hh/ME-1-a" || ts.Head() != head {
		t.Errorf("result = %+v", r)
	}
}

// A branch of the task from an earlier title is still the branch of the task.
func TestStartFindsTheBranchOfAnEarlierTitle(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("new title", nil)
	dir := codeRepo(t)
	addBranch(t, dir, "hh/ME-1-old-title")
	r, err := integration.Start(ts.Session, "ME-1", dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Branch != "hh/ME-1-old-title" || r.Created || !r.Switch {
		t.Errorf("result = %+v", r)
	}
	if hasBranch(t, dir, "hh/ME-1-new-title") {
		t.Error("a second branch was made")
	}
}

// A prefix that changed leaves the task with a branch of its former id; an id
// in other letters is the same id, as the fetch reads a branch.
func TestStartFindsTheBranchOfAFormerID(t *testing.T) {
	t.Parallel()
	for _, branch := range []string{"hh/OLD-1-a", "hh/old-1-a", "hh/me-1-a"} {
		ts := usecasetest.New(t)
		ts.AddTask("a", func(x *model.Task) { x.FormerIDs = []string{"OLD-1"} })
		dir := codeRepo(t)
		addBranch(t, dir, branch)
		r, err := integration.Start(ts.Session, "ME-1", dir)
		if err != nil {
			t.Fatal(err)
		}
		if r.Branch != branch || r.Created || !r.Switch {
			t.Errorf("%s: result = %+v", branch, r)
		}
	}
}

func TestStartOnADetachedHead(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("a", nil)
	ts.AddTask("b", nil)
	dir := codeRepo(t)
	detach(t, dir)
	r, err := integration.Start(ts.Session, "ME-1", dir)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Created || r.Switch || branchAt(t, dir) != "refs/heads/hh/ME-1-a" {
		t.Errorf("no branch yet: result = %+v, HEAD = %s", r, branchAt(t, dir))
	}
	addBranch(t, dir, "hh/ME-2-b")
	detach(t, dir)
	r, err = integration.Start(ts.Session, "ME-2", dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Created || !r.Switch || r.Branch != "hh/ME-2-b" {
		t.Errorf("a branch there: result = %+v", r)
	}
}

func TestStartOnAnotherBranchDoesNotSwitch(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("a", nil)
	dir := codeRepo(t)
	addBranch(t, dir, "hh/ME-1-a")
	r, err := integration.Start(ts.Session, "ME-1", dir)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Switch || r.Created || r.Branch != "hh/ME-1-a" || !r.Committed {
		t.Fatalf("result = %+v", r)
	}
	if got := branchAt(t, dir); got != "refs/heads/main" {
		t.Errorf("HEAD = %s", got)
	}
	if x := taskOf(t, ts.Session); x.Status != "in_progress" || len(x.Assignees) != 1 {
		t.Errorf("task = %s %v", x.Status, x.Assignees)
	}
}

func TestStartDoesNotTakeTheBranchOfAnotherTask(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("a", nil)
	dir := codeRepo(t)
	addBranch(t, dir, "hh/ME-12-a")
	r, err := integration.Start(ts.Session, "ME-1", dir)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Created || r.Branch != "hh/ME-1-a" {
		t.Errorf("result = %+v", r)
	}
}

func TestStartOutsideARepository(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("a", nil)
	head := ts.Head()
	_, err := integration.Start(ts.Session, "ME-1", t.TempDir())
	usecasetest.MustCode(t, err, usecase.KindInvalid, "code.no_repository")
	if ts.Head() != head {
		t.Error("the task was written")
	}
}

func TestStartOfARepositoryWithNoCommit(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("a", nil)
	dir := t.TempDir()
	if _, err := git.PlainInit(dir, false); err != nil {
		t.Fatal(err)
	}
	head := ts.Head()
	_, err := integration.Start(ts.Session, "ME-1", dir)
	usecasetest.MustCode(t, err, usecase.KindInvalid, "code.no_commit")
	if ts.Head() != head {
		t.Error("the task was written")
	}
}

func TestStartOfADeletedTask(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("a", nil)
	ts.DeleteTask("ME-1")
	dir := codeRepo(t)
	if _, err := integration.Start(ts.Session, "ME-1", dir); err == nil {
		t.Fatal("a deleted task was started")
	}
	if got := branchAt(t, dir); got != "refs/heads/main" {
		t.Errorf("HEAD = %s", got)
	}
}

// The workflow refuses todo → in_progress: the branch stays, and nothing of
// the write does.
func TestStartRefusedKeepsTheBranch(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("a", nil)
	sc, _ := ts.StoreSchema()
	sc.Workflow.Transitions = schema.Lists{{Key: "todo", Values: []string{"done"}}}
	ts.MustEdit("me", "schema", usecasetest.TAuthor, func(tx *usecasetest.Tx) error { return tx.PutSchema(sc) })
	dir := codeRepo(t)
	_, err := integration.Start(ts.Session, "ME-1", dir)
	var ve *store.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v", err)
	}
	if got := branchAt(t, dir); got != "refs/heads/hh/ME-1-a" {
		t.Errorf("HEAD = %s", got)
	}
	if x := taskOf(t, ts.Session); x.Status != "todo" || len(x.Assignees) != 0 {
		t.Errorf("task = %s %v", x.Status, x.Assignees)
	}
}

func TestStartLeavesADoingStatus(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("a", func(x *model.Task) { x.Status = "waiting" })
	if _, err := integration.Start(ts.Session, "ME-1", codeRepo(t)); err != nil {
		t.Fatal(err)
	}
	if x := taskOf(t, ts.Session); x.Status != "waiting" || len(x.Assignees) != 1 {
		t.Errorf("task = %s %v", x.Status, x.Assignees)
	}
}

func TestStartOfATaskOfAnotherStore(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	id := usecasetest.AddProjectIn(t, ts, "org-b", "in b")
	r, err := integration.Start(ts.Session, id, codeRepo(t))
	if err != nil || !r.Committed {
		t.Fatalf("result = %+v, %v", r, err)
	}
}
