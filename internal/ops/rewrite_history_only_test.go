package ops

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

// D34: a task whose forward was purged is still a target of the rewrite, as
// long as its file is in the history of the store.
func TestRewriteHistoryAfterPurgeMoved(t *testing.T) {
	t.Parallel()
	f := newRewriteFx(t, false)
	f.moved(t)
	if _, _, err := f.org().PurgeMoved(true); err != nil {
		t.Fatal(err)
	}
	f.sync(t)
	// the old id is gone with the forward; the new one names the task
	newID := mustTaskID(t, f.ts, "org-b", f.p)

	// without --store: the store whose history has the task
	plan, err := f.ts.PrepareRewrite(RewriteInput{Arg: newID})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Store != "org-a" || len(plan.Tasks) != 3 {
		t.Fatalf("plan = %+v, want the three tasks of org-a", plan)
	}
	for _, x := range plan.Tasks {
		if x.Live {
			t.Errorf("%s is not live", x.ULID)
		}
	}
	res, err := f.ts.RewriteHistory(RewriteInput{Arg: newID, Typed: "org-a", Confirmed: plan.ULIDs()})
	if err != nil {
		t.Fatal(err)
	}
	if res.Rewritten == 0 {
		t.Errorf("res = %+v", res)
	}
	r := f.repo(t)
	head, _ := r.Head()
	noTrace(t, r, head, []model.ULID{f.p, f.c, f.g}, f.commentRel, f.worklogRel)
	// org-b keeps them
	if _, err := f.ts.ForStore("org-b").Resolve(mustIndex(t, f.ts, "org-b"), newID); err != nil {
		t.Error(err)
	}
}

func TestRewriteHistoryAfterPurgeMovedWithStore(t *testing.T) {
	t.Parallel()
	f := newRewriteFx(t, false)
	f.moved(t)
	if _, _, err := f.org().PurgeMoved(true); err != nil {
		t.Fatal(err)
	}
	f.sync(t)
	newID := mustTaskID(t, f.ts, "org-b", f.p)
	plan, err := f.org().PrepareRewrite(RewriteInput{Arg: newID})
	if err != nil || plan.Store != "org-a" || len(plan.Tasks) != 3 {
		t.Fatalf("plan = %+v, %v", plan, err)
	}
}

// tasksInHistory answers for several tasks in one pass: the live, the moved
// whose forward is purged, and one that never was there.
func TestTasksInHistorySeveralULIDs(t *testing.T) {
	t.Parallel()
	f := newRewriteFx(t, false)
	f.moved(t)
	if _, _, err := f.org().PurgeMoved(true); err != nil {
		t.Fatal(err)
	}
	never := model.NewULID()
	got, err := tasksInHistory(f.repo(t), []model.ULID{f.p, f.c, f.g, f.first, f.o, never})
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []model.ULID{f.p, f.c, f.g, f.first, f.o} {
		if !got[u] {
			t.Errorf("%s is not found", u)
		}
	}
	if got[never] {
		t.Error("a task that never was there is found")
	}
	if got, err := tasksInHistory(f.repo(t), nil); err != nil || len(got) != 0 {
		t.Errorf("no ULIDs: %v, %v", got, err)
	}
}

// A task file that was only ever on the merged-in side of a merge (the second
// parent) is found, whether the side deleted it before the merge or the
// merge deleted it.
func TestTasksInHistoryOnTheMergedInSide(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := gitrepo.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	author := gitrepo.Signature{Name: "T", Email: "t@example.com"}
	now := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	write := func(rel string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	remove := func(rel string) {
		t.Helper()
		if err := os.Remove(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(subject string, parents []plumbing.Hash, paths ...string) plumbing.Hash {
		t.Helper()
		h, err := r.Commit(gitrepo.Commit{Paths: paths, Parents: parents, Subject: subject, Author: author, When: now})
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	deletedBySide, deletedByMerge, never := model.NewULID(), model.NewULID(), model.NewULID()
	pa, pb := model.TaskPath(deletedBySide), model.TaskPath(deletedByMerge)
	write("base.md")
	base := commit("base", nil, "base.md")
	// the side: adds both, then drops the first
	write(pa)
	write(pb)
	s1 := commit("side adds", []plumbing.Hash{base}, pa, pb)
	remove(pa)
	s2 := commit("side drops one", []plumbing.Hash{s1}, pa)
	// the main line, from base: neither file, one other file
	remove(pb)
	write("other.md")
	m1 := commit("main", []plumbing.Hash{base}, pb, "other.md")
	// the merge has neither file, so it deletes the second from the side's tree
	commit("merge", []plumbing.Hash{m1, s2})

	got, err := tasksInHistory(r, []model.ULID{deletedBySide, deletedByMerge, never})
	if err != nil {
		t.Fatal(err)
	}
	if !got[deletedBySide] || !got[deletedByMerge] || got[never] {
		t.Errorf("found %v; want the two of the side and not the third", got)
	}
}

// With two stores that have the task in their history, the user picks.
func TestRewriteHistoryTwoStoresHaveItInTheHistory(t *testing.T) {
	t.Parallel()
	f := newMoveFx(t)
	c, p, err := f.ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	acct, _ := c.Account("alice")
	author, err := config.CommitAuthor(acct, true, f.ts.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(p.StoresDir, "org-c")
	ws := &model.Workspace{ULID: model.NewULID(), Name: "org-c", IDPrefix: "CCC"}
	if _, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "org-c"}, schema.Default(), ws, author, "", f.ts.Now); err != nil {
		t.Fatal(err)
	}
	if err := c.AddStore("org-c", dir, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.SetStoreAccount("org-c", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	// org-a -> org-c -> org-b, and the forwards purged
	if _, err := f.ts.MoveStore(MoveStoreInput{Arg: "API-2", To: "org-c", Yes: true}); err != nil {
		t.Fatal(err)
	}
	id := mustTaskID(t, f.ts, "org-c", f.p)
	if _, err := f.ts.MoveStore(MoveStoreInput{Arg: "org-c:" + id, To: "org-b", Yes: true}); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"org-a", "org-c"} {
		if _, _, err := f.ts.ForStore(n).PurgeMoved(true); err != nil {
			t.Fatal(err)
		}
	}
	newID := mustTaskID(t, f.ts, "org-b", f.p)
	_, err = f.ts.PrepareRewrite(RewriteInput{Arg: newID})
	var e *Error
	if !errors.As(err, &e) || e.Code != "choice.required" {
		t.Fatalf("err = %v, want choice.required", err)
	}
}
