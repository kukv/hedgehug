package ops

import (
	"os"
	"testing"
)

func TestCommentAddAfterTaskDeletedWhileEditing(t *testing.T) {
	t.Parallel()
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
	p, err := ts.PrepareComment(cs, "late")
	if err != nil {
		t.Fatal(err)
	}
	r, err := ts.WriteComment(p)
	if err != nil || !r.Committed || r.Comment.Body != "late\n" {
		t.Fatalf("WriteComment = %+v, %v", r, err)
	}
	_, err = ts.CommentAdd("ME-1", "again")
	wantErr(t, err, KindConflict, "task.deleted")
}

func TestPrepareErrorsAreReturnedBare(t *testing.T) {
	t.Parallel()
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
	ces, err := ts.OpenComment(c.Ref.Comment.String())
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
	ts.resetWriter()
	_, _, boom := ts.Writer()
	if boom == nil {
		t.Fatal("Writer read a config that does not parse")
	}
	bare := func(name string, err error) {
		t.Helper()
		if err == nil || err.Error() != boom.Error() || AsError(err).Code != "" || KindOf(err) != KindUnknown {
			t.Errorf("%s = %v; want the config error itself (%v)", name, err, boom)
		}
	}
	_, err = ts.PrepareComment(cs, "x")
	bare("PrepareComment", err)
	bare("PrepareEdit", ts.PrepareEdit(es, es.Old))
	_, err = ts.PrepareCommentEdit(ces, "two\n")
	bare("PrepareCommentEdit", err)
	// A failed read is not kept: with the config back, the writer is read.
	if err := os.WriteFile(p.ConfigFile, good, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ts.Writer(); err != nil {
		t.Errorf("Writer after the config is back = %v; want the error not to be cached", err)
	}
	// Nothing to read for an empty body, a body that stays, or a broken task.
	if _, err := ts.PrepareComment(cs, " "); err == nil || err.Error() != "the comment is empty" {
		t.Errorf("PrepareComment of white space = %v", err)
	}
	if _, err := ts.PrepareCommentEdit(ces, "one\n"); err != nil {
		t.Errorf("PrepareCommentEdit of the same body = %v", err)
	}
	old2 := es.Old
	es.Old = nil
	if err := ts.PrepareEdit(es, old2); err != nil {
		t.Errorf("PrepareEdit of a broken task = %v", err)
	}
}
