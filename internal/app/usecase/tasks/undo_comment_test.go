package tasks_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/comments"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func liveCommentCount(t *testing.T, ts *usecasetest.Session, task model.ULID) int {
	t.Helper()
	s, _, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for c, err := range s.Comments(task) {
		if err != nil {
			t.Fatal(err)
		}
		if c.DeletedAt == nil {
			n++
		}
	}
	return n
}

func TestUndoOfAnUnpushedCommentStillWorks(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AppendConfig(usecasetest.AliceAccount)
	res, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := res.View()
	if err != nil {
		t.Fatal(err)
	}
	task, err := model.ParseULID(v.ULID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := comments.CommentAdd(ts.Session, "ME-1", "hello"); err != nil {
		t.Fatal(err)
	}
	if n := liveCommentCount(t, ts, task); n != 1 {
		t.Fatalf("comments = %d, want 1", n)
	}
	u, err := tasks.Undo(ts.Session)
	if err != nil || !u.Found || !u.Changed {
		t.Fatalf("Undo = %+v, %v", u, err)
	}
	if n := liveCommentCount(t, ts, task); n != 0 {
		t.Errorf("comments after undo = %d, want 0", n)
	}
}

func TestUndoRefusesToTakeBackAPushedComment(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AppendConfig(usecasetest.AliceAccount)
	res, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := res.View()
	if err != nil {
		t.Fatal(err)
	}
	task, err := model.ParseULID(v.ULID)
	if err != nil {
		t.Fatal(err)
	}
	c, err := comments.CommentAdd(ts.Session, "ME-1", "hello")
	if err != nil {
		t.Fatal(err)
	}
	// The push records which comment of GitHub this is. It is a commit of its
	// own that undo passes over, so the undo reaches the adding.
	s, _, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Update(s, store.Op{Name: usecase.OpGitHubPush, Automated: true, Task: task}, func(tx *store.Tx) error {
		cc, err := comments.FindComment(s, c.Ref)
		if err != nil {
			return err
		}
		if err := cc.SetGitHub([]model.CommentLink{{Ref: "github.com/o/r/issues/1", ID: 42, BodySynced: "hello"}}); err != nil {
			return err
		}
		return tx.PutCommentGH(task, cc)
	}); err != nil {
		t.Fatal(err)
	}
	_, err = tasks.Undo(ts.Session)
	usecasetest.WantErr(t, err, usecase.KindInvalid, "undo.comment_pushed")
	if n := liveCommentCount(t, ts, task); n != 1 {
		t.Errorf("comments after the refused undo = %d, want 1", n)
	}
	// hedgehug comment rm still works.
	if _, err := comments.CommentRemove(ts.Session, c.Ref.Comment.String()); err != nil {
		t.Errorf("comment rm: %v", err)
	}
}
