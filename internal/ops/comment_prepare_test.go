package ops

import (
	"errors"
	"testing"
)

func TestCommentAddAfterTaskDeletedWhileEditing(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	cs, err := ts.BeginComment("ME-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Remove("ME-1"); err != nil {
		t.Fatal(err)
	}
	// The task was checked once, before the body was taken; deleting it since
	// does not refuse the comment.
	r, err := ts.CommentAddTo(cs, "late")
	if err != nil || !r.Committed || r.Comment.Body != "late\n" {
		t.Fatalf("CommentAddTo = %+v, %v", r, err)
	}
	_, err = ts.CommentAdd("ME-1", "again")
	wantErr(t, err, KindConflict, "task.deleted")
}

func TestPrepareErrorsAreReturnedBare(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	cs, err := ts.BeginComment("ME-1")
	if err != nil {
		t.Fatal(err)
	}
	c, err := ts.CommentAdd("ME-1", "one")
	if err != nil {
		t.Fatal(err)
	}
	es, err := ts.BeginEdit("ME-1")
	if err != nil {
		t.Fatal(err)
	}
	s, ref, old, err := ts.OpenComment(c.Ref.Comment.String())
	if err != nil {
		t.Fatal(err)
	}
	// The writer cannot be read: the errors before a write carry no more
	// than the error itself.
	boom := errors.New("boom")
	ts.writerDone, ts.writerErr = true, boom
	if _, err := ts.PrepareComment(cs, "x"); err != boom {
		t.Errorf("PrepareComment = %v; want boom itself", err)
	}
	if err := ts.PrepareEdit(es, es.Old); err != boom {
		t.Errorf("PrepareEdit = %v; want boom itself", err)
	}
	if _, err := ts.PrepareCommentEdit(s, ref, old, "two\n"); err != boom {
		t.Errorf("PrepareCommentEdit = %v; want boom itself", err)
	}
	// Nothing to read for an empty body, a body that stays, or a broken task.
	if _, err := ts.PrepareComment(cs, " "); err == nil || err.Error() != "the comment is empty" {
		t.Errorf("PrepareComment of white space = %v", err)
	}
	if _, err := ts.PrepareCommentEdit(s, ref, old, "one\n"); err != nil {
		t.Errorf("PrepareCommentEdit of the same body = %v", err)
	}
	old2 := es.Old
	es.Old = nil
	if err := ts.PrepareEdit(es, old2); err != nil {
		t.Errorf("PrepareEdit of a broken task = %v", err)
	}
}
