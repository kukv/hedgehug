package ops

import (
	"strings"
	"testing"
)

func TestCommentAddEditRemove(t *testing.T) {
	ts := newTestSession(t)
	ts.asAccount("alice")
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	c, err := ts.CommentAdd("ME-1", "first")
	if err != nil || !c.Committed {
		t.Fatalf("CommentAdd = %+v, %v", c, err)
	}
	if c.Comment.Body != "first\n" || c.Ref.Comment != c.Comment.ULID {
		t.Errorf("CommentAdd comment = %+v ref %+v", c.Comment, c.Ref)
	}
	id := c.Ref.Comment.String()
	r, err := ts.CommentEdit(id, "second\n")
	if err != nil || !r.Committed || r.Comment.Body != "second\n" {
		t.Fatalf("CommentEdit = %+v, %v", r, err)
	}
	ces, err := ts.OpenComment(id)
	if err != nil {
		t.Fatal(err)
	}
	if got := ces.Comment; CommentBody(got.Body) != "second\n" {
		t.Errorf("body = %q; want second", got.Body)
	}
	rm, err := ts.CommentRemove(id)
	if err != nil || !rm.Committed || rm.Comment.DeletedAt == nil || !rm.Comment.DeletedAt.Equal(ts.Now()) {
		t.Fatalf("CommentRemove = %+v, %v", rm, err)
	}
	if _, err := ts.CommentAdd("ME-1", ""); KindOf(err) != KindInvalid {
		t.Errorf("empty comment kind = %v; want KindInvalid", KindOf(err))
	}
	if _, err := ts.OpenComment("01JZZZZZZZZZZZZZZZZZZZZZZZ"); KindOf(err) != KindNotFound {
		t.Errorf("unknown comment kind = %v; want KindNotFound", KindOf(err))
	}
}

func TestCommentBody(t *testing.T) {
	for in, want := range map[string]string{
		"":              "",
		" \t\r\n":       "",
		"x":             "x\n",
		"x  \n\n":       "x\n",
		"  x\ny\r\n":    "  x\ny\n",
		"a\n\nb":        "a\n\nb\n",
		"\n\nleading\n": "\n\nleading\n",
	} {
		if got := CommentBody(in); got != want {
			t.Errorf("CommentBody(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestCommentAddErrors(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	_, err := ts.CommentAdd("ME-1", " \n")
	wantErr(t, err, KindInvalid, "comment.empty")
	if err != nil && err.Error() != "the comment is empty" {
		t.Errorf("message = %q", err)
	}
	_, err = ts.CommentAdd("ME-99", "x")
	wantErr(t, err, KindNotFound, "task.not_found")
	if _, err := ts.Remove("ME-1"); err != nil {
		t.Fatal(err)
	}
	_, err = ts.CommentAdd("ME-1", "x")
	wantErr(t, err, KindConflict, "task.deleted")
}

func TestOpenCommentErrors(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	for _, arg := range []string{"abc", "01JZZZZZZZZZZZZZZZZZZZZZZZ"} {
		_, err := ts.OpenComment(arg)
		wantErr(t, err, KindNotFound, "comment.not_found")
		var e *Error
		if !asError(err, &e) || e.Args["input"] != arg {
			t.Errorf("Args = %+v; want input %q", e, arg)
		}
	}
	_, err := ts.OpenComment("abc")
	if want := `comment "abc": give the full ULID of the comment: not found`; err == nil || err.Error() != want {
		t.Errorf("message = %v; want %q", err, want)
	}
	_, err = ts.OpenComment("01JZZZZZZZZZZZZZZZZZZZZZZZ")
	if want := `comment "01JZZZZZZZZZZZZZZZZZZZZZZZ": not found`; err == nil || err.Error() != want {
		t.Errorf("message = %v; want %q", err, want)
	}
}

func TestOpenCommentIsCaseInsensitive(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	c, err := ts.CommentAdd("ME-1", "x")
	if err != nil {
		t.Fatal(err)
	}
	ces, err := ts.OpenComment(strings.ToLower(c.Ref.Comment.String()))
	if err != nil || ces.Ref != c.Ref || ces.Comment.Body != "x\n" || ces.Store == nil {
		t.Errorf("OpenComment = %+v, %v", ces, err)
	}
}

func TestFindCommentNotFound(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	c, err := ts.CommentAdd("ME-1", "x")
	if err != nil {
		t.Fatal(err)
	}
	ref := c.Ref
	ref.Comment = c.Ref.Task // a ULID that names no comment of the task
	_, err = FindComment(c.Store, ref)
	wantErr(t, err, KindNotFound, "comment.not_found")
}

func TestCommentEditCases(t *testing.T) {
	ts := newTestSession(t)
	ts.asAccount("alice")
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	c, err := ts.CommentAdd("ME-1", "one")
	if err != nil {
		t.Fatal(err)
	}
	id := c.Ref.Comment.String()
	same, err := ts.CommentEdit(id, "one\n")
	if err != nil || same.Committed {
		t.Errorf("same body = %+v, %v; want no commit", same, err)
	}
	_, err = ts.CommentEdit(id, "")
	wantErr(t, err, KindInvalid, "comment.empty")
	if want := "the comment is empty; delete it with hedgehug comment rm " + id; err != nil && err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
	// The comment changed since the caller read it.
	ces, err := ts.OpenComment(id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ts.CommentEdit(id, "two\n"); err != nil {
		t.Fatal(err)
	}
	pce, err := ts.PrepareCommentEdit(ces, "three\n")
	if err != nil {
		t.Fatal(err)
	}
	_, err = ts.WriteCommentEdit(pce)
	wantErr(t, err, KindConflict, "comment.changed")
	if err != nil && err.Error() != "the comment changed while editing" {
		t.Errorf("stale WriteCommentEdit = %v; want the comment changed while editing", err)
	}
	// A body of white space only is refused whoever cleaned it.
	_, err = ts.CommentEdit(id, " \n\t")
	wantErr(t, err, KindInvalid, "comment.empty")
	if _, err := ts.CommentRemove(id); err != nil {
		t.Fatal(err)
	}
	_, err = ts.CommentEdit(id, "four\n")
	wantErr(t, err, KindConflict, "comment.deleted")
	if want := "comment " + id + " is deleted and cannot be edited"; err != nil && err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
	_, err = ts.CommentRemove(id)
	wantErr(t, err, KindConflict, "comment.deleted")
	if want := "comment " + id + " is already deleted"; err != nil && err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
	_, err = ts.CommentEdit("abc", "x\n")
	wantErr(t, err, KindNotFound, "comment.not_found")
	_, err = ts.CommentRemove("abc")
	wantErr(t, err, KindNotFound, "comment.not_found")
}
