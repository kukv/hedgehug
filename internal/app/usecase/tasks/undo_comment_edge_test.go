package tasks_test

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/comments"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// pushedComment adds a comment as alice and records a gh on it, as a push
// does (an operation undo passes over).
func pushedComment(t *testing.T) (ts *usecasetest.Session, task model.ULID, c comments.CommentResult, s *store.Store) {
	t.Helper()
	ts = usecasetest.New(t)
	ts.AppendConfig(usecasetest.AliceAccount)
	res, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := res.View()
	if err != nil {
		t.Fatal(err)
	}
	if task, err = model.ParseULID(v.ULID); err != nil {
		t.Fatal(err)
	}
	if c, err = comments.CommentAdd(ts.Session, "ME-1", "hello"); err != nil {
		t.Fatal(err)
	}
	if s, _, err = ts.Open(); err != nil {
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
	return ts, task, c, s
}

func TestUndoOfAPushedCommentDeletedSinceDoesNotPointAtCommentRm(t *testing.T) {
	t.Parallel()
	ts, task, c, s := pushedComment(t)
	// Deleted by an operation undo passes over, as a mirror or a repair does.
	if _, err := ts.Update(s, store.Op{Name: "repair", Task: task}, func(tx *store.Tx) error {
		cc, err := comments.FindComment(s, c.Ref)
		if err != nil {
			return err
		}
		now := cc.Created
		cc.DeletedAt = &now
		return tx.PutComment(task, cc)
	}); err != nil {
		t.Fatal(err)
	}
	_, err := tasks.Undo(ts.Session)
	usecasetest.WantErr(t, err, usecase.KindInvalid, "undo.comment_pushed")
	if strings.Contains(err.Error(), "comment rm") || !strings.Contains(err.Error(), "deleted") {
		t.Errorf("message = %q; want one for a deleted comment, with no comment rm", err)
	}
}

func TestUndoPassesOverACommentFileThatDoesNotRead(t *testing.T) {
	t.Parallel()
	ts, task, c, s := pushedComment(t)
	if _, err := ts.Update(s, store.Op{Name: "repair", Task: task}, func(tx *store.Tx) error {
		return tx.PutFile(model.CommentPath(task, c.Ref.Comment), []byte("not a comment\n"))
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Undo(ts.Session); err != nil {
		t.Errorf("undo with a comment file that does not read: %v", err)
	}
}
