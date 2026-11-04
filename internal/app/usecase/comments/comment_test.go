package comments_test

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/comments"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestCommentAddEditRemove(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AsAccount("alice")
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	c, err := comments.CommentAdd(ts.Session, "ME-1", "first")
	if err != nil || !c.Committed {
		t.Fatalf("CommentAdd = %+v, %v", c, err)
	}
	if c.Comment.Body != "first\n" || c.Ref.Comment != c.Comment.ULID {
		t.Errorf("CommentAdd comment = %+v ref %+v", c.Comment, c.Ref)
	}
	id := c.Ref.Comment.String()
	r, err := comments.CommentEdit(ts.Session, id, "second\n")
	if err != nil || !r.Committed || r.Comment.Body != "second\n" {
		t.Fatalf("CommentEdit = %+v, %v", r, err)
	}
	ces, err := comments.OpenComment(ts.Session, id)
	if err != nil {
		t.Fatal(err)
	}
	if got := ces.Comment; comments.CommentBody(got.Body) != "second\n" {
		t.Errorf("body = %q; want second", got.Body)
	}
	rm, err := comments.CommentRemove(ts.Session, id)
	if err != nil || !rm.Committed || rm.Comment.DeletedAt == nil || !rm.Comment.DeletedAt.Equal(ts.Now()) {
		t.Fatalf("CommentRemove = %+v, %v", rm, err)
	}
	if _, err := comments.CommentAdd(ts.Session, "ME-1", ""); usecase.KindOf(err) != usecase.KindInvalid {
		t.Errorf("empty comment kind = %v; want KindInvalid", usecase.KindOf(err))
	}
	if _, err := comments.OpenComment(ts.Session, "01JZZZZZZZZZZZZZZZZZZZZZZZ"); usecase.KindOf(err) != usecase.KindNotFound {
		t.Errorf("unknown comment kind = %v; want KindNotFound", usecase.KindOf(err))
	}
}

func TestCommentBody(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"":              "",
		" \t\r\n":       "",
		"x":             "x\n",
		"x  \n\n":       "x\n",
		"  x\ny\r\n":    "  x\ny\n",
		"a\n\nb":        "a\n\nb\n",
		"\n\nleading\n": "\n\nleading\n",
	} {
		if got := comments.CommentBody(in); got != want {
			t.Errorf("CommentBody(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestCommentAddErrors(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	_, err := comments.CommentAdd(ts.Session, "ME-1", " \n")
	usecasetest.WantErr(t, err, usecase.KindInvalid, "comment.empty")
	if err != nil && err.Error() != "the comment is empty" {
		t.Errorf("message = %q", err)
	}
	_, err = comments.CommentAdd(ts.Session, "ME-99", "x")
	usecasetest.WantErr(t, err, usecase.KindNotFound, "task.not_found")
	if _, err := tasks.Remove(ts.Session, "ME-1"); err != nil {
		t.Fatal(err)
	}
	_, err = comments.CommentAdd(ts.Session, "ME-1", "x")
	usecasetest.WantErr(t, err, usecase.KindConflict, "task.deleted")
}

func TestOpenCommentErrors(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	for _, arg := range []string{"abc", "01JZZZZZZZZZZZZZZZZZZZZZZZ"} {
		_, err := comments.OpenComment(ts.Session, arg)
		usecasetest.WantErr(t, err, usecase.KindNotFound, "comment.not_found")
		var e *usecase.Error
		if !usecasetest.AsError(err, &e) || e.Args["input"] != arg {
			t.Errorf("Args = %+v; want input %q", e, arg)
		}
	}
	_, err := comments.OpenComment(ts.Session, "abc")
	if want := `comment "abc": give the full ULID of the comment: not found`; err == nil || err.Error() != want {
		t.Errorf("message = %v; want %q", err, want)
	}
	_, err = comments.OpenComment(ts.Session, "01JZZZZZZZZZZZZZZZZZZZZZZZ")
	if want := `comment "01JZZZZZZZZZZZZZZZZZZZZZZZ": not found`; err == nil || err.Error() != want {
		t.Errorf("message = %v; want %q", err, want)
	}
}

func TestOpenCommentIsCaseInsensitive(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	c, err := comments.CommentAdd(ts.Session, "ME-1", "x")
	if err != nil {
		t.Fatal(err)
	}
	ces, err := comments.OpenComment(ts.Session, strings.ToLower(c.Ref.Comment.String()))
	if err != nil || ces.Ref != c.Ref || ces.Comment.Body != "x\n" || ces.Store == nil {
		t.Errorf("OpenComment = %+v, %v", ces, err)
	}
}

func TestFindCommentNotFound(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	c, err := comments.CommentAdd(ts.Session, "ME-1", "x")
	if err != nil {
		t.Fatal(err)
	}
	ref := c.Ref
	ref.Comment = c.Ref.Task // a ULID that names no comment of the task
	_, err = comments.FindComment(c.Store, ref)
	usecasetest.WantErr(t, err, usecase.KindNotFound, "comment.not_found")
}

func TestCommentEditCases(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AsAccount("alice")
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	c, err := comments.CommentAdd(ts.Session, "ME-1", "one")
	if err != nil {
		t.Fatal(err)
	}
	id := c.Ref.Comment.String()
	same, err := comments.CommentEdit(ts.Session, id, "one\n")
	if err != nil || same.Committed {
		t.Errorf("same body = %+v, %v; want no commit", same, err)
	}
	_, err = comments.CommentEdit(ts.Session, id, "")
	usecasetest.WantErr(t, err, usecase.KindInvalid, "comment.empty")
	if want := "the comment is empty; delete it with hedgehug comment rm " + id; err != nil && err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
	// The comment changed since the caller read it.
	ces, err := comments.OpenComment(ts.Session, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := comments.CommentEdit(ts.Session, id, "two\n"); err != nil {
		t.Fatal(err)
	}
	pce, err := comments.PrepareCommentEdit(ts.Session, ces, "three\n")
	if err != nil {
		t.Fatal(err)
	}
	_, err = comments.WriteCommentEdit(ts.Session, pce)
	usecasetest.WantErr(t, err, usecase.KindConflict, "comment.changed")
	if err != nil && err.Error() != "the comment changed while editing" {
		t.Errorf("stale WriteCommentEdit = %v; want the comment changed while editing", err)
	}
	// A body of white space only is refused whoever cleaned it.
	_, err = comments.CommentEdit(ts.Session, id, " \n\t")
	usecasetest.WantErr(t, err, usecase.KindInvalid, "comment.empty")
	if _, err := comments.CommentRemove(ts.Session, id); err != nil {
		t.Fatal(err)
	}
	_, err = comments.CommentEdit(ts.Session, id, "four\n")
	usecasetest.WantErr(t, err, usecase.KindConflict, "comment.deleted")
	if want := "comment " + id + " is deleted and cannot be edited"; err != nil && err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
	_, err = comments.CommentRemove(ts.Session, id)
	usecasetest.WantErr(t, err, usecase.KindConflict, "comment.deleted")
	if want := "comment " + id + " is already deleted"; err != nil && err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
	_, err = comments.CommentEdit(ts.Session, "abc", "x\n")
	usecasetest.WantErr(t, err, usecase.KindNotFound, "comment.not_found")
	_, err = comments.CommentRemove(ts.Session, "abc")
	usecasetest.WantErr(t, err, usecase.KindNotFound, "comment.not_found")
}

func TestCommentAddCleansTheBody(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AsAccount("alice")
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	_, err := comments.CommentAdd(ts.Session, "ME-1", " \t\r\n")
	usecasetest.WantErr(t, err, usecase.KindInvalid, "comment.empty")
	r, err := comments.CommentAdd(ts.Session, "ME-1", "raw  \n\n")
	if err != nil || r.Comment.Body != "raw\n" || len(r.Commit) != 40 {
		t.Errorf("CommentAdd = %+v, %v; want the body cleaned and a commit hash", r, err)
	}
	// An edit takes a raw body too, and stores it cleaned.
	e, err := comments.CommentEdit(ts.Session, r.Ref.Comment.String(), "again \n\n")
	if err != nil || e.Comment.Body != "again\n" || len(e.Commit) != 40 {
		t.Errorf("CommentEdit = %+v, %v", e, err)
	}
	same, err := comments.CommentEdit(ts.Session, r.Ref.Comment.String(), "again")
	if err != nil || same.Committed || same.Commit != "" {
		t.Errorf("CommentEdit of the same body = %+v, %v; want no commit", same, err)
	}
}
