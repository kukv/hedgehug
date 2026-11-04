package comments_test

import (
	"os"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/comments"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestCommentAddAfterTaskDeletedWhileEditing(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	cs, err := comments.BeginComment(ts.Session, "ME-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Remove(ts.Session, "ME-1"); err != nil {
		t.Fatal(err)
	}
	// The task was checked once, before the body was taken; deleting it since
	// does not refuse the comment.
	p, err := comments.PrepareComment(ts.Session, cs, "late")
	if err != nil {
		t.Fatal(err)
	}
	r, err := comments.WriteComment(ts.Session, p)
	if err != nil || !r.Committed || r.Comment.Body != "late\n" {
		t.Fatalf("WriteComment = %+v, %v", r, err)
	}
	_, err = comments.CommentAdd(ts.Session, "ME-1", "again")
	usecasetest.WantErr(t, err, usecase.KindConflict, "task.deleted")
}

func TestPrepareErrorsAreReturnedBare(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	cs, err := comments.BeginComment(ts.Session, "ME-1")
	if err != nil {
		t.Fatal(err)
	}
	c, err := comments.CommentAdd(ts.Session, "ME-1", "one")
	if err != nil {
		t.Fatal(err)
	}
	es, err := tasks.BeginEdit(ts.Session, "ME-1")
	if err != nil {
		t.Fatal(err)
	}
	ces, err := comments.OpenComment(ts.Session, c.Ref.Comment.String())
	if err != nil {
		t.Fatal(err)
	}
	// The writer cannot be read (config.yaml does not parse): the errors
	// before a write carry no more than the error itself.
	_, p, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	good, err := os.ReadFile(p.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.ConfigFile, []byte("stores: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ts.ForgetWriter()
	_, _, boom := ts.Writer()
	if boom == nil {
		t.Fatal("Writer read a config that does not parse")
	}
	bare := func(name string, err error) {
		t.Helper()
		if err == nil || err.Error() != boom.Error() || usecase.AsError(err).Code != "" || usecase.KindOf(err) != usecase.KindUnknown {
			t.Errorf("%s = %v; want the config error itself (%v)", name, err, boom)
		}
	}
	_, err = comments.PrepareComment(ts.Session, cs, "x")
	bare("PrepareComment", err)
	bare("PrepareEdit", tasks.PrepareEdit(ts.Session, es, es.Old))
	_, err = comments.PrepareCommentEdit(ts.Session, ces, "two\n")
	bare("PrepareCommentEdit", err)
	// A failed read is not kept: with the config back, the writer is read.
	if err := os.WriteFile(p.ConfigFile, good, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ts.Writer(); err != nil {
		t.Errorf("Writer after the config is back = %v; want the error not to be cached", err)
	}
	// Nothing to read for an empty body, a body that stays, or a broken task.
	if _, err := comments.PrepareComment(ts.Session, cs, " "); err == nil || err.Error() != "the comment is empty" {
		t.Errorf("PrepareComment of white space = %v", err)
	}
	if _, err := comments.PrepareCommentEdit(ts.Session, ces, "one\n"); err != nil {
		t.Errorf("PrepareCommentEdit of the same body = %v", err)
	}
	old2 := es.Old
	es.Old = nil
	if err := tasks.PrepareEdit(ts.Session, es, old2); err != nil {
		t.Errorf("PrepareEdit of a broken task = %v", err)
	}
}
