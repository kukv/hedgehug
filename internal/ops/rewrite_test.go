package ops

import (
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// rewriteFx is moveFx with a remote for org-a (and a mirror when asked), the
// stores synced.
type rewriteFx struct {
	*moveFx
	bare, mirror string
}

func newRewriteFx(t *testing.T, withMirror bool) *rewriteFx {
	t.Helper()
	f := &rewriteFx{moveFx: newMoveFx(t)}
	f.bare = filepath.Join(t.TempDir(), "origin.git")
	remotes := []string{f.bare}
	if err := gitrepo.InitBare(f.bare); err != nil {
		t.Fatal(err)
	}
	if withMirror {
		f.mirror = filepath.Join(t.TempDir(), "mirror.git")
		if err := gitrepo.InitBare(f.mirror); err != nil {
			t.Fatal(err)
		}
		remotes = append(remotes, f.mirror)
	}
	c, _, err := f.ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	e, err := c.Select("org-a")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveStore("org-a"); err != nil {
		t.Fatal(err)
	}
	if err := c.AddStore("org-a", e.Path, remotes); err != nil {
		t.Fatal(err)
	}
	if err := c.SetStoreAccount("org-a", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	f.sync(t)
	return f
}

func (f *rewriteFx) org() *Session { return f.ts.ForStore("org-a") }

func (f *rewriteFx) sync(t *testing.T) {
	t.Helper()
	if _, err := f.org().Sync(false, true); err != nil {
		t.Fatal(err)
	}
}

// moved moves the project API-2 and what is under it to org-b and syncs
// org-a, so that origin has the forwards.
func (f *rewriteFx) moved(t *testing.T) {
	t.Helper()
	if _, err := f.ts.MoveStore(f.in()); err != nil {
		t.Fatal(err)
	}
	f.sync(t)
}

// originHead is the head of main in the bare repository of origin.
func originHead(t *testing.T, bare string) plumbing.Hash {
	t.Helper()
	r, err := git.PlainOpen(bare)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := r.Reference("refs/heads/main", true)
	if err != nil {
		t.Fatal(err)
	}
	return ref.Hash()
}

func (f *rewriteFx) repo(t *testing.T) *gitrepo.Repo {
	t.Helper()
	s, err := f.org().OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	return s.Repo()
}

// noTrace checks that no commit of the history up to head has a file of the
// tasks, and that rels are gone too.
func noTrace(t *testing.T, r *gitrepo.Repo, head plumbing.Hash, ulids []model.ULID, rels ...string) {
	t.Helper()
	for _, u := range ulids {
		rels = append(rels, model.TaskPath(u))
	}
	cs, err := r.Between(head, plumbing.ZeroHash)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cs {
		for _, rel := range rels {
			if _, err := r.ReadAt(c.Hash, rel); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("%s at %s (%s): %v; want gone", rel, c.Hash, c.Subject, err)
			}
		}
	}
}

func subjects(t *testing.T, r *gitrepo.Repo) []string {
	t.Helper()
	l, err := r.Log(nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(l))
	for i, e := range l {
		out[i] = e.Subject
	}
	return out
}

func TestRewriteHistoryOfMovedTasks(t *testing.T) {
	f := newRewriteFx(t, true)
	f.moved(t)
	r := f.repo(t)
	before, err := r.Log(nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	api1 := f.first
	oldHead := originHead(t, f.bare)
	ixBefore := mustIndex(t, f.ts, "org-a")
	if _, ok := ixBefore.Forward(f.p); !ok {
		t.Fatal("no forward before")
	}

	res, err := f.org().RewriteHistory(RewriteInput{Arg: "API-2", Typed: "org-a"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Plan.Store != "org-a" || len(res.Plan.Tasks) != 3 || res.Rewritten == 0 || len(res.Pushed) != 2 || len(res.Warnings) != 0 {
		t.Fatalf("res = %+v", res)
	}
	for _, x := range res.Plan.Tasks {
		if x.Live {
			t.Errorf("%s is a forward, not live", x.ULID)
		}
	}

	r = f.repo(t)
	head, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	ulids := []model.ULID{f.p, f.c, f.g}
	noTrace(t, r, head, ulids, f.commentRel, f.worklogRel)
	// origin and the mirror have it
	for _, bare := range []string{f.bare, f.mirror} {
		if got := originHead(t, bare); got != head {
			t.Errorf("%s is at %s; want %s", bare, got, head)
		}
	}
	if oldHead == head {
		t.Error("the head did not change")
	}
	// the tasks that stay keep their files; the subjects lost the titles
	if _, err := r.ReadAt(head, model.TaskPath(api1)); err != nil {
		t.Errorf("API-1: %v", err)
	}
	var purged, keptTitle int
	for _, s := range subjects(t, r) {
		if strings.HasSuffix(s, " (purged)") {
			purged++
		}
		if strings.Contains(s, "API-1 ") {
			keptTitle++
		}
	}
	if purged == 0 || keptTitle == 0 {
		t.Errorf("purged %d, kept %d in %v", purged, keptTitle, subjects(t, r))
	}
	// authors and times of the commits made again stay
	after, err := r.Log(nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before)+1 {
		t.Fatalf("%d commits after, %d before", len(after), len(before))
	}
	for i := range before {
		b, a := before[i], after[i+1]
		if b.Author != a.Author || !b.When.Equal(a.When) {
			t.Errorf("commit %d: %v %v, was %v %v", i, a.Author, a.When, b.Author, b.When)
		}
	}
	// the record
	data, err := r.ReadAt(head, model.RewritesPath)
	if err != nil {
		t.Fatal(err)
	}
	rs, err := model.DecodeRewrites(data)
	if err != nil || len(rs) != 1 || rs[0].Before != oldHead.String() || len(rs[0].ULIDs) != 3 || !rs[0].At.Equal(f.ts.Now().UTC().Truncate(time.Second)) {
		t.Errorf("rewrites.yaml = %+v, %v", rs, err)
	}
	if after[0].Subject != "hedgehug: rewrite-history 3 tasks" {
		t.Errorf("subject = %q", after[0].Subject)
	}
	// the work tree has no file of them and the index follows the new head
	s, err := f.org().OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range ulids {
		if _, err := os.Stat(filepath.Join(s.Dir(), filepath.FromSlash(model.TaskPath(u)))); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s: %v", u, err)
		}
	}
	if st, err := r.Status(); err != nil || len(st) != 0 {
		t.Errorf("status = %v, %v", st, err)
	}
	if err := ixBefore.Refresh(); err != nil {
		t.Fatal(err)
	}
	if _, ok := ixBefore.Forward(f.p); ok {
		t.Error("the index still has the forward")
	}
	if _, ok := ixBefore.Task(api1); !ok {
		t.Error("the index lost API-1")
	}
	// no progress file, a sync goes through and has nothing to do
	if _, err := s.ReadLocal(rewriteProgressFile); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("progress file: %v", err)
	}
	reps, err := f.org().Sync(false, true)
	if err != nil || len(reps) != 1 || len(reps[0].Warnings) != 0 {
		t.Errorf("Sync = %+v, %v", reps, err)
	}
	if got, _ := r.Head(); got != head {
		t.Errorf("the sync moved the head to %s", got)
	}
	// the moved tasks live on in org-b
	if _, err := f.ts.ForStore("org-b").Resolve(mustIndex(t, f.ts, "org-b"), "WEB-3"); err != nil {
		t.Error(err)
	}
}

func TestRewriteHistoryOfALiveTask(t *testing.T) {
	f := newRewriteFx(t, false)
	a := f.org()
	secret, err := a.Add(AddInput{Title: "Secret"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := a.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	cid, wid := model.NewULID(), model.NewULID()
	if _, err := a.Update(s, store.Op{Name: "comment", Task: secret.ULID}, func(tx *store.Tx) error {
		if err := tx.PutComment(secret.ULID, &model.Comment{ULID: cid, Author: "github.com/alice", Body: "hidden words"}); err != nil {
			return err
		}
		return tx.PutWorklog(secret.ULID, &model.Worklog{ULID: wid, Author: "github.com/alice", Duration: 60, Body: "work"})
	}); err != nil {
		t.Fatal(err)
	}
	f.sync(t)
	ix := mustIndex(t, f.ts, "org-a")
	e, err := a.Resolve(ix, "API-6")
	if err != nil || e.ULID != secret.ULID {
		t.Fatalf("API-6 = %v, %v", e, err)
	}

	plan, err := a.PrepareRewrite(RewriteInput{Arg: "API-6"})
	if err != nil || len(plan.Tasks) != 1 || !plan.Tasks[0].Live || plan.Tasks[0].ID != "API-6" || plan.Tasks[0].Title != "Secret" {
		t.Fatalf("plan = %+v, %v", plan, err)
	}
	res, err := a.RewriteHistory(RewriteInput{Arg: "API-6", Typed: "org-a", Confirmed: plan.ULIDs()})
	if err != nil {
		t.Fatal(err)
	}
	r := f.repo(t)
	head, _ := r.Head()
	noTrace(t, r, head, []model.ULID{secret.ULID}, model.CommentPath(secret.ULID, cid), model.WorklogPath(secret.ULID, wid))
	if got := originHead(t, f.bare); got != head || len(res.Pushed) != 1 {
		t.Errorf("origin %s, head %s, pushed %v", got, head, res.Pushed)
	}
	// the purge commit went first, with the number kept
	subs := subjects(t, r)
	if !strings.Contains(strings.Join(subs, "\n"), "hedgehug: purge API-6 (purged)") {
		t.Errorf("subjects = %v", subs)
	}
	nums, err := r.ReadAt(head, model.NumbersPath)
	if err != nil {
		t.Fatal(err)
	}
	n, err := model.DecodeNumbers(nums)
	if err != nil || n["API"] != 6 {
		t.Errorf("numbers = %v, %v", n, err)
	}
	next, err := a.Add(AddInput{Title: "Next"})
	if err != nil {
		t.Fatal(err)
	}
	if got := mustTaskID(t, f.ts, "org-a", next.ULID); got != "API-7" {
		t.Errorf("the next id = %s", got)
	}
}

func mustTaskID(t *testing.T, ts *testSession, name string, u model.ULID) string {
	t.Helper()
	e, ok := mustIndex(t, ts, name).Task(u)
	if !ok {
		t.Fatalf("no task %s in %s", u, name)
	}
	return e.ID
}

func TestRewriteHistoryAsksForTheStoreName(t *testing.T) {
	f := newRewriteFx(t, false)
	f.moved(t)
	r := f.repo(t)
	head, _ := r.Head()

	_, err := f.org().RewriteHistory(RewriteInput{Arg: "API-2"})
	mustCode(t, err, KindUsage, "confirm.required")
	args := argsOf(err)
	if plan, ok := args["plan"].(RewritePlan); !ok || len(plan.Tasks) != 3 || plan.Store != "org-a" {
		t.Errorf("args = %v", args)
	}
	_, err = f.org().RewriteHistory(RewriteInput{Arg: "API-2", Typed: "org-b"})
	mustCode(t, err, KindInvalid, "rewrite.canceled")
	if err.Error() != "Canceled" {
		t.Errorf("message = %q", err)
	}
	_, err = f.org().RewriteHistory(RewriteInput{Arg: "API-2", Typed: "org-a", Confirmed: []model.ULID{f.p}})
	mustCode(t, err, KindConflict, "rewrite.changed")
	if got, _ := r.Head(); got != head {
		t.Errorf("a refused rewrite moved the head to %s", got)
	}
	if got := originHead(t, f.bare); got != head {
		t.Errorf("origin = %s", got)
	}
	if _, err := os.Stat(f.org().rewriteProgressPath(t)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("progress file: %v", err)
	}
}

func (ss *Session) rewriteProgressPath(t *testing.T) string {
	t.Helper()
	s, err := ss.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	return s.LocalPath(rewriteProgressFile)
}

func TestRewriteHistoryRefusals(t *testing.T) {
	f := newRewriteFx(t, false)
	f.moved(t)
	a := f.org()

	// nothing of the task in the store named
	_, err := f.ts.ForStore("org-a").RewriteHistory(RewriteInput{Arg: "WEB-1", Typed: "org-a"})
	mustCode(t, err, KindNotFound, "rewrite.nothing")

	// an uncommitted change
	s, err := a.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	stray := filepath.Join(s.Dir(), "stray.txt")
	if err := os.WriteFile(stray, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = a.PrepareRewrite(RewriteInput{Arg: "API-2"})
	mustCode(t, err, KindConflict, "rewrite.dirty")
	_, err = a.RewriteHistory(RewriteInput{Arg: "API-2", Typed: "org-a"})
	mustCode(t, err, KindConflict, "rewrite.dirty")
	if err := os.Remove(stray); err != nil {
		t.Fatal(err)
	}

	// a read-only store
	if err := sync.WriteAccess(s, sync.Access{ReadOnly: true, Reason: sync.ReasonGitHub, Checked: f.ts.Now()}); err != nil {
		t.Fatal(err)
	}
	_, err = a.PrepareRewrite(RewriteInput{Arg: "API-2"})
	mustCode(t, err, KindConflict, "store.read_only")
	_, err = a.RewriteHistory(RewriteInput{Arg: "API-2", Typed: "org-a"})
	mustCode(t, err, KindConflict, "store.read_only")
}

func TestRewriteHistoryNeedsARemote(t *testing.T) {
	f := newMoveFx(t)
	if _, err := f.ts.MoveStore(f.in()); err != nil {
		t.Fatal(err)
	}
	_, err := f.ts.ForStore("org-a").PrepareRewrite(RewriteInput{Arg: "API-2"})
	mustCode(t, err, KindInvalid, "rewrite.no_remote")
}

// A change made after the sync is not pushed: the rewrite under the locks
// refuses, and so does one that finds origin moved on.
func TestRewriteHistoryRechecksUnderTheLocks(t *testing.T) {
	f := newRewriteFx(t, false)
	f.moved(t)
	a := f.org()
	run, err := a.locateRewrite("API-2")
	if err != nil {
		t.Fatal(err)
	}
	if err := run.prepare("API-2"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Add(AddInput{Title: "later"}); err != nil {
		t.Fatal(err)
	}
	_, err = run.execute()
	mustCode(t, err, KindConflict, "rewrite.unpushed")
	f.sync(t)

	// origin moves on: another clone pushes a commit
	run, err = a.locateRewrite("API-2")
	if err != nil {
		t.Fatal(err)
	}
	if err := run.prepare("API-2"); err != nil {
		t.Fatal(err)
	}
	other := cloneSessionOf(t, f)
	if _, err := other.Add(AddInput{Title: "from elsewhere"}); err != nil {
		t.Fatal(err)
	}
	if _, err := other.Sync(false, true); err != nil {
		t.Fatal(err)
	}
	head, _ := f.repo(t).Head()
	_, err = run.execute()
	mustCode(t, err, KindConflict, "rewrite.origin_moved")
	if got, _ := f.repo(t).Head(); got != head {
		t.Errorf("the head moved to %s", got)
	}
	if _, err := os.Stat(a.rewriteProgressPath(t)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("progress file: %v", err)
	}
}

// cloneSessionOf is a session of another clone of org-a.
func cloneSessionOf(t *testing.T, f *rewriteFx) *testSession {
	t.Helper()
	c := otherSession(t, f.ts, f.ts.Now())
	if _, err := c.StoreAdd("org-a", []string{f.bare}, "", ""); err != nil {
		t.Fatal(err)
	}
	return c
}

// The push to origin fails: the history is made again here, the file is
// kept, a sync is refused, and running it again pushes only.
func TestRewriteHistoryResumesAfterAFailedPush(t *testing.T) {
	f := newRewriteFx(t, false)
	f.moved(t)
	gone := f.bare + ".gone"
	f.ts.BeforeUpdate = func() { // the commit of the record; the push follows
		if err := os.Rename(f.bare, gone); err != nil {
			t.Fatal(err)
		}
	}
	oldHead := originHead(t, f.bare)
	a := f.org()
	_, err := a.RewriteHistory(RewriteInput{Arg: "API-2", Typed: "org-a"})
	if err == nil || !strings.Contains(err.Error(), "run purge --rewrite-history again") {
		t.Fatalf("err = %v", err)
	}
	f.ts.BeforeUpdate = nil
	r := f.repo(t)
	head, _ := r.Head()
	noTrace(t, r, head, []model.ULID{f.p, f.c, f.g}, f.commentRel)
	if _, err := os.Stat(a.rewriteProgressPath(t)); err != nil {
		t.Fatalf("progress file: %v", err)
	}

	// a sync is refused while it stays
	if err := os.Rename(gone, f.bare); err != nil {
		t.Fatal(err)
	}
	if got := originHead(t, f.bare); got != oldHead {
		t.Fatalf("origin = %s", got)
	}
	_, err = a.Sync(false, true)
	mustCode(t, err, KindConflict, "rewrite.in_progress")
	if !strings.Contains(err.Error(), "a history rewrite of org-a is unfinished; run hedgehug purge --rewrite-history again") {
		t.Errorf("err = %v", err)
	}

	// the plan of the resume is the recorded one, and the task is not found by its id any more
	plan, err := a.PrepareRewrite(RewriteInput{Arg: "API-2"})
	if err != nil || !plan.Resumed || len(plan.Tasks) != 3 {
		t.Fatalf("plan = %+v, %v", plan, err)
	}
	commits := len(subjects(t, r))
	res, err := a.RewriteHistory(RewriteInput{Arg: "API-2", Typed: "org-a"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Rewritten != 0 || len(res.Pushed) != 1 || !res.Plan.Resumed {
		t.Errorf("res = %+v", res)
	}
	if got := originHead(t, f.bare); got != head {
		t.Errorf("origin = %s; want %s", got, head)
	}
	if n := len(subjects(t, f.repo(t))); n != commits {
		t.Errorf("%d commits; want %d", n, commits)
	}
	if _, err := os.Stat(a.rewriteProgressPath(t)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("progress file: %v", err)
	}
	if _, err := a.Sync(false, true); err != nil {
		t.Errorf("sync after: %v", err)
	}
}

// The run stopped between making the history again and the record: it goes
// on from the record, without making a second history.
func TestRewriteHistoryResumesBeforeTheRecord(t *testing.T) {
	f := newRewriteFx(t, false)
	f.moved(t)
	// a run that failed right after the record file was written
	fail := errors.New("stop")
	f.ts.BeforeUpdate = func() { panic(fail) }
	func() {
		defer func() {
			if recover() == nil {
				t.Error("the hook did not run")
			}
		}()
		_, _ = f.org().RewriteHistory(RewriteInput{Arg: "API-2", Typed: "org-a"})
	}()
	f.ts.BeforeUpdate = nil
	a := f.org()
	r := f.repo(t)
	head, _ := r.Head()
	// ResetMain was done: HEAD has no trace but the record is not there yet
	noTrace(t, r, head, []model.ULID{f.p, f.c, f.g}, f.commentRel)
	if _, err := r.ReadAt(head, model.RewritesPath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("rewrites.yaml: %v", err)
	}
	if _, err := os.Stat(a.rewriteProgressPath(t)); err != nil {
		t.Fatalf("progress file: %v", err)
	}
	res, err := a.RewriteHistory(RewriteInput{Arg: "API-2", Typed: "org-a"})
	if err != nil {
		t.Fatal(err)
	}
	after := f.repo(t)
	h2, _ := after.Head()
	if got := originHead(t, f.bare); got != h2 || len(res.Pushed) != 1 {
		t.Errorf("origin %s, head %s, pushed %v", got, h2, res.Pushed)
	}
	data, err := after.ReadAt(h2, model.RewritesPath)
	if err != nil {
		t.Fatal(err)
	}
	if rs, err := model.DecodeRewrites(data); err != nil || len(rs) != 1 {
		t.Errorf("rewrites.yaml = %+v, %v", rs, err)
	}
	noTrace(t, after, h2, []model.ULID{f.p, f.c, f.g}, f.commentRel)
}

// A mirror that fails is a warning; origin has the history.
func TestRewriteHistoryMirrorFailureIsAWarning(t *testing.T) {
	f := newRewriteFx(t, true)
	f.moved(t)
	gone := f.mirror + ".gone"
	f.ts.BeforeUpdate = func() {
		if err := os.Rename(f.mirror, gone); err != nil {
			t.Fatal(err)
		}
	}
	res, err := f.org().RewriteHistory(RewriteInput{Arg: "API-2", Typed: "org-a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Pushed) != 1 || len(res.Warnings) != 1 || !strings.HasPrefix(res.Warnings[0], "push mirror1: ") {
		t.Errorf("res = %+v", res)
	}
	head, _ := f.repo(t).Head()
	if got := originHead(t, f.bare); got != head {
		t.Errorf("origin = %s", got)
	}
	s, err := f.org().OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadLocal(rewriteProgressFile); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("progress file: %v", err)
	}
}

func TestRewriteMessage(t *testing.T) {
	a, b, other := model.NewULID(), model.NewULID(), model.NewULID()
	set := map[model.ULID]bool{a: true, b: true}
	drop := rewriteDrop(set)
	msg := strings.Join([]string{
		"hedgehug: update API-2 Secret title",
		"",
		"Hedgehug-Op: update",
		"Hedgehug-Task: " + a.String(),
		"Hedgehug-Overwritten: " + url.PathEscape(model.TaskPath(a)) + " title \"x\" github.com/bob",
		"Hedgehug-Overwritten: " + url.PathEscape(model.TaskPath(other)) + " title \"y\" github.com/bob",
		"Hedgehug-Overwritten: " + url.PathEscape(model.CommentPath(b, other)) + " * \"z\" -",
		"Hedgehug-Repaired: " + b.String() + " parent_deleted {} -",
		"Hedgehug-Repaired: " + other.String() + " parent_deleted {} -",
		"",
	}, "\n")
	got := rewriteMessage(set, drop)(msg)
	want := strings.Join([]string{
		"hedgehug: update API-2 (purged)",
		"",
		"Hedgehug-Op: update",
		"Hedgehug-Task: " + a.String(),
		"Hedgehug-Overwritten: " + url.PathEscape(model.TaskPath(other)) + " title \"y\" github.com/bob",
		"Hedgehug-Repaired: " + other.String() + " parent_deleted {} -",
		"",
	}, "\n")
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	// a commit about another task keeps its title; a subject with no title is as it was
	unrelated := "hedgehug: add API-9 Title\n\nHedgehug-Task: " + other.String() + "\n"
	if m := rewriteMessage(set, drop)(unrelated); m != unrelated {
		t.Errorf("changed: %q", m)
	}
	spaced := "hedgehug: add API-2 A title with  several words\n\nHedgehug-Task: " + a.String() + "\n"
	if m := rewriteMessage(set, drop)(spaced); m != "hedgehug: add API-2 (purged)\n\nHedgehug-Task: "+a.String()+"\n" {
		t.Errorf("spaced title: %q", m)
	}
	foreign := "Merge branch x\n\nHedgehug-Task: " + a.String() + "\n"
	if m := rewriteMessage(set, drop)(foreign); m != foreign {
		t.Errorf("a subject that is not ours changed: %q", m)
	}
	bare := "hedgehug: rm " + a.String() + "\n\nHedgehug-Task: " + a.String() + "\n"
	if m := rewriteMessage(set, drop)(bare); m != bare {
		t.Errorf("changed: %q", m)
	}
	// the message of a commit made again once is as it was
	if m := rewriteMessage(set, drop)(want); m != want {
		t.Errorf("not idempotent: %q", m)
	}
}

func TestRewriteDrop(t *testing.T) {
	a, other := model.NewULID(), model.NewULID()
	drop := rewriteDrop(map[model.ULID]bool{a: true})
	for p, want := range map[string]bool{
		model.TaskPath(a):                  true,
		model.TaskPath(other):              false,
		model.CommentPath(a, other):        true,
		model.WorklogPath(a, other):        true,
		model.CommentPath(other, a):        false,
		"workspaces/" + a.String() + ".md": false,
		"schema.yaml":                      false,
		".hedgehug/rewrites.yaml":          false,
	} {
		if got := drop(p); got != want {
			t.Errorf("drop(%s) = %v", p, got)
		}
	}
}

// Someone pushes to origin between the failed push and the resume: the
// resume refuses and changes nothing.
func TestRewriteHistoryResumeRefusesWhenOriginMoved(t *testing.T) {
	f := newRewriteFx(t, false)
	f.moved(t)
	gone := f.bare + ".gone"
	f.ts.BeforeUpdate = func() {
		if err := os.Rename(f.bare, gone); err != nil {
			t.Fatal(err)
		}
	}
	a := f.org()
	if _, err := a.RewriteHistory(RewriteInput{Arg: "API-2", Typed: "org-a"}); err == nil {
		t.Fatal("the push did not fail")
	}
	f.ts.BeforeUpdate = nil
	if err := os.Rename(gone, f.bare); err != nil {
		t.Fatal(err)
	}
	other := cloneSessionOf(t, f)
	if _, err := other.Add(AddInput{Title: "from elsewhere"}); err != nil {
		t.Fatal(err)
	}
	if _, err := other.Sync(false, true); err != nil {
		t.Fatal(err)
	}
	pushed := originHead(t, f.bare)
	head, _ := f.repo(t).Head()
	_, err := a.RewriteHistory(RewriteInput{Arg: "API-2", Typed: "org-a"})
	mustCode(t, err, KindConflict, "rewrite.origin_moved")
	if got := originHead(t, f.bare); got != pushed {
		t.Errorf("origin = %s; want %s", got, pushed)
	}
	if got, _ := f.repo(t).Head(); got != head {
		t.Errorf("the head moved to %s", got)
	}
	if _, err := os.Stat(a.rewriteProgressPath(t)); err != nil {
		t.Errorf("progress file: %v", err)
	}
}

// A run stops before or right after the commit of step 5: the file is
// there already, and the next run does what is left.
func TestRewriteHistoryResumesAroundStepFive(t *testing.T) {
	for _, stopAt := range []int{1, 2} { // the purge commit, the record commit
		f := newRewriteFx(t, false)
		secret, err := f.org().Add(AddInput{Title: "Secret"})
		if err != nil {
			t.Fatal(err)
		}
		f.sync(t)
		n := 0
		f.ts.BeforeUpdate = func() {
			if n++; n == stopAt {
				panic("stop")
			}
		}
		func() {
			defer func() {
				if recover() == nil {
					t.Error("the hook did not run")
				}
			}()
			_, _ = f.org().RewriteHistory(RewriteInput{Arg: "API-6", Typed: "org-a"})
		}()
		f.ts.BeforeUpdate = nil
		a := f.org()
		if _, err := os.Stat(a.rewriteProgressPath(t)); err != nil {
			t.Fatalf("stop %d: progress file: %v", stopAt, err)
		}
		if _, err := a.Sync(false, true); AsError(err) == nil || AsError(err).Code != "rewrite.in_progress" {
			t.Fatalf("stop %d: sync = %v", stopAt, err)
		}
		if _, err := a.RewriteHistory(RewriteInput{Arg: "API-6", Typed: "org-a"}); err != nil {
			t.Fatalf("stop %d: %v", stopAt, err)
		}
		r := f.repo(t)
		head, _ := r.Head()
		noTrace(t, r, head, []model.ULID{secret.ULID})
		if got := originHead(t, f.bare); got != head {
			t.Errorf("stop %d: origin = %s", stopAt, got)
		}
		nums, err := r.ReadAt(head, model.NumbersPath)
		if err != nil {
			t.Fatal(err)
		}
		if nm, err := model.DecodeNumbers(nums); err != nil || nm["API"] != 6 {
			t.Errorf("stop %d: numbers = %v, %v", stopAt, nm, err)
		}
		if !strings.Contains(strings.Join(subjects(t, r), "\n"), "hedgehug: purge API-6 (purged)") {
			t.Errorf("stop %d: no purge commit: %v", stopAt, subjects(t, r))
		}
	}
}

// A broken progress file is refused by the sync and by a new run alike,
// naming the file.
func TestRewriteProgressFileBroken(t *testing.T) {
	f := newRewriteFx(t, false)
	f.moved(t)
	a := f.org()
	file := a.rewriteProgressPath(t)
	for _, content := range []string{"not json", `{"before":"","ulids":[]}`} {
		if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := a.Sync(false, true)
		mustCode(t, err, KindInvalid, "rewrite.progress_unreadable")
		if !strings.Contains(err.Error(), file) {
			t.Errorf("sync error does not name the file: %v", err)
		}
		_, err = a.RewriteHistory(RewriteInput{Arg: "API-2", Typed: "org-a"})
		mustCode(t, err, KindInvalid, "rewrite.progress_unreadable")
		if !strings.Contains(err.Error(), file) {
			t.Errorf("run error does not name the file: %v", err)
		}
	}
}

// What a purged live task leaves in the tasks outside the set: a reference
// to it stays, as plain purge leaves it.
func TestRewriteHistoryDanglingReference(t *testing.T) {
	f := newRewriteFx(t, false)
	a := f.org()
	secret, err := a.Add(AddInput{Title: "Secret"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := a.Add(AddInput{Title: "Other"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Dep("API-7", "API-6", true); err != nil {
		t.Fatal(err)
	}
	f.sync(t)
	if _, err := a.RewriteHistory(RewriteInput{Arg: "API-6", Typed: "org-a"}); err != nil {
		t.Fatal(err)
	}
	s, err := a.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	o, err := s.Task(other.ULID)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("blocked_by after the purge: %v (purged %s)", o.BlockedBy, secret.ULID)
}
