package usecase_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/comments"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/app/usecase/workspaces"
)

func TestResultsCarryTheCommitAndAFreshView(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AsAccount("alice")
	head := func() string {
		t.Helper()
		s, err := ts.OpenStore()
		if err != nil {
			t.Fatal(err)
		}
		h, err := s.Repo().Head()
		if err != nil {
			t.Fatal(err)
		}
		return h.String()
	}
	add, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"})
	if err != nil || add.Commit == "" || add.Commit != head() {
		t.Fatalf("Add = %+v, %v; want Commit = HEAD %s", add, err, head())
	}
	// The index of the result was read before the write; View reads again.
	if usecase.ResultIndexHas(add, add.ULID) {
		t.Fatal("the index of Add already holds the task")
	}
	if v, err := add.View(); err != nil || v.ID != "ME-1" || v.Title != "a" {
		t.Errorf("Add View = %+v, %v", v, err)
	}
	set, err := tasks.Set(ts.Session, "ME-1", []usecase.Assign{{Key: "title", Op: "=", Value: "b"}})
	if err != nil || set.Commit != head() || set.Commit == add.Commit {
		t.Fatalf("Set = %+v, %v; want a new Commit = HEAD", set, err)
	}
	if v, err := set.View(); err != nil || v.Title != "b" {
		t.Errorf("Set View = %+v, %v; want title b", v, err)
	}
	// Nothing changed: no commit, and HEAD stays.
	before := head()
	noop, err := tasks.Set(ts.Session, "ME-1", []usecase.Assign{{Key: "title", Op: "=", Value: "b"}})
	if err != nil || noop.Committed || noop.Commit != "" || head() != before {
		t.Errorf("no-op Set = %+v, %v", noop, err)
	}
	if v, err := noop.View(); err != nil || v.Title != "b" {
		t.Errorf("no-op View = %+v, %v", v, err)
	}

	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "c", Parent: "ME-1"}); err != nil {
		t.Fatal(err)
	}
	rm, err := tasks.Remove(ts.Session, "ME-1")
	if err != nil || rm.Commit != head() {
		t.Fatalf("Remove = %+v, %v", rm, err)
	}
	vs, err := rm.Views()
	if err != nil || len(vs) != 2 {
		t.Fatalf("Remove Views = %d, %v; want 2", len(vs), err)
	}
	for _, v := range vs {
		if v.DeletedAt == nil {
			t.Errorf("Views of Remove: %s is not deleted", v.ID)
		}
	}
	rs, err := tasks.Restore(ts.Session, "ME-1")
	if err != nil || rs.Commit != head() {
		t.Fatalf("Restore = %+v, %v", rs, err)
	}
	if vs, err := rs.Views(); err != nil || len(vs) != 2 || vs[0].DeletedAt != nil || vs[1].DeletedAt != nil {
		t.Errorf("Restore Views = %+v, %v; want both live", vs, err)
	}

	cm, err := comments.CommentAdd(ts.Session, "ME-1", "x")
	if err != nil || cm.Commit != head() {
		t.Errorf("CommentAdd = %+v, %v; want Commit = HEAD", cm, err)
	}
	rc, err := comments.CommentRemove(ts.Session, cm.Ref.Comment.String())
	if err != nil || rc.Commit != head() {
		t.Errorf("CommentRemove = %+v, %v; want Commit = HEAD", rc, err)
	}
	mv, err := tasks.Move(ts.Session, tasks.MoveInput{Arg: "ME-2", Root: true})
	if err != nil || !mv.Committed || mv.Commit != head() {
		t.Errorf("Move = %+v, %v; want Commit = HEAD", mv, err)
	}
	ws, err := workspaces.WorkspaceAdd(ts.Session, "other", "OT")
	if err != nil || ws.Commit != head() {
		t.Errorf("WorkspaceAdd = %+v, %v; want Commit = HEAD", ws, err)
	}
	ts.UseAccount()
	if _, err := tasks.Set(ts.Session, "ME-1", []usecase.Assign{{Key: "title", Op: "=", Value: "z"}}); err != nil {
		t.Fatal(err)
	}
	un, err := tasks.Undo(ts.Session)
	if err != nil || un.NewCommit != head() {
		t.Errorf("Undo = %+v, %v; want NewCommit = HEAD", un, err)
	}
}
