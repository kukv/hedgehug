package tasks_test

import (
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/comments"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestCheckCommentEditable(t *testing.T) {
	t.Parallel()
	ref := store.CommentRef{}
	me := []string{"github.com/Alice"}
	c := &model.Comment{ULID: model.NewULID(), Author: "github.com/alice"}
	if err := comments.CheckCommentEditable(ref, c, me, "edited"); err != nil {
		t.Errorf("own comment: %v", err)
	}
	c.Author = "github.com/bob"
	err := comments.CheckCommentEditable(ref, c, me, "removed")
	usecasetest.WantErr(t, err, usecase.KindConflict, "comment.not_author")
	if want := "github.com/bob's comment cannot be removed"; err != nil && err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
	c.Author = ""
	err = comments.CheckCommentEditable(ref, c, me, "edited")
	usecasetest.WantErr(t, err, usecase.KindConflict, "comment.no_author")
	if want := "a comment whose author is unknown cannot be edited"; err != nil && err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
	c.Author = "github.com/bob"
	now := time.Now()
	c.DeletedAt = &now
	usecasetest.WantErr(t, comments.CheckCommentEditable(ref, c, me, "edited"), usecase.KindConflict, "comment.deleted")
}

func TestCheckCommentEditableRefusesAMirroredComment(t *testing.T) {
	t.Parallel()
	ref := store.CommentRef{}
	me := []string{"github.com/alice"}
	c := &model.Comment{ULID: model.NewULID(), Author: "github:octocat"}
	for _, verb := range []string{"edited", "removed"} {
		err := comments.CheckCommentEditable(ref, c, me, verb)
		usecasetest.WantErr(t, err, usecase.KindInvalid, "comment.mirrored")
		if want := "octocat's comment was taken from GitHub and cannot be " + verb + "; change it on GitHub"; err != nil && err.Error() != want {
			t.Errorf("message = %q; want %q", err, want)
		}
	}
	// Even an author in me: the prefix is checked before the author.
	usecasetest.WantErr(t, comments.CheckCommentEditable(ref, c, []string{"github:octocat"}, "edited"), usecase.KindInvalid, "comment.mirrored")
	// Deleted comes first.
	now := time.Now()
	c.DeletedAt = &now
	usecasetest.WantErr(t, comments.CheckCommentEditable(ref, c, me, "edited"), usecase.KindConflict, "comment.deleted")
}

func TestCommentAuthorOps(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	ts.AsAccount("alice")
	c, err := comments.CommentAdd(ts.Session, "ME-1", "mine")
	if err != nil || c.Comment.Author != "github.com/alice" {
		t.Fatalf("CommentAdd = %+v, %v", c.Comment, err)
	}
	id := c.Ref.Comment.String()
	if _, err := comments.CommentEdit(ts.Session, id, "edited"); err != nil {
		t.Errorf("own edit: %v", err)
	}
	ts.AsAccount("bob")
	_, err = comments.CommentEdit(ts.Session, id, "hijack")
	usecasetest.WantErr(t, err, usecase.KindConflict, "comment.not_author")
	_, err = comments.CommentRemove(ts.Session, id)
	usecasetest.WantErr(t, err, usecase.KindConflict, "comment.not_author")
	ts.AsAccount("Alice")
	if _, err := comments.CommentRemove(ts.Session, id); err != nil {
		t.Errorf("own rm: %v", err)
	}
	// Deleted and someone else's: the deleted error comes first.
	ts.AsAccount("bob")
	_, err = comments.CommentRemove(ts.Session, id)
	usecasetest.WantErr(t, err, usecase.KindConflict, "comment.deleted")
	_, err = comments.CommentEdit(ts.Session, id, "x")
	usecasetest.WantErr(t, err, usecase.KindConflict, "comment.deleted")
}

func TestCommentWithoutAuthor(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	r, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"})
	if err != nil {
		t.Fatal(err)
	}
	// A comment file with no author, as an older store may hold one.
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	cu := model.NewULID()
	if _, err := ts.Update(s, store.Op{Name: "comment", Task: r.ULID}, func(tx *store.Tx) error {
		return tx.PutComment(r.ULID, &model.Comment{ULID: cu, Body: "nobody\n"})
	}); err != nil {
		t.Fatal(err)
	}
	_, err = comments.CommentEdit(ts.Session, cu.String(), "x")
	usecasetest.WantErr(t, err, usecase.KindConflict, "comment.no_author")
	_, err = comments.CommentRemove(ts.Session, cu.String())
	usecasetest.WantErr(t, err, usecase.KindConflict, "comment.no_author")
}

func TestMentionWarnings(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AsAccount("alice")
	ts.SetMembers(replicate.Members{Collaborators: []replicate.Collaborator{{ID: "github.com/bob", Permission: "write"}}})
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "t"}); err != nil {
		t.Fatal(err)
	}
	edit := func(body string) {
		t.Helper()
		es, err := tasks.BeginEdit(ts.Session, "ME-1")
		if err != nil {
			t.Fatal(err)
		}
		es.Old.Body = body
		b, err := es.Old.Encode()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tasks.Edit(ts.Session, "ME-1", b); err != nil {
			t.Fatal(err)
		}
	}
	want := func(label string, ws ...string) {
		t.Helper()
		if !slices.Equal(ts.Warns, ws) {
			t.Errorf("%s: warnings = %q; want %q", label, ts.Warns, ws)
		}
		ts.Warns = nil
	}
	edit("hi @bob and @carol\n")
	want("task edit", "carol cannot read me")
	edit("hi @bob and @carol\nmore text\n")
	want("carol was there")
	edit("hi @github.com/carol @carol @Github.com/Bob\n")
	want("carol was there, bob reads")
	edit("@github.com/dave @dave\n")
	want("the same login once", "dave cannot read me")

	c, err := comments.CommentAdd(ts.Session, "ME-1", "ping @bob @carol")
	if err != nil {
		t.Fatal(err)
	}
	want("comment add", "carol cannot read me")
	id := c.Ref.Comment.String()
	if _, err := comments.CommentEdit(ts.Session, id, "ping @bob @carol\nand @erin"); err != nil {
		t.Fatal(err)
	}
	want("comment edit", "erin cannot read me")

	ts.SetMembers(replicate.Members{})
	if _, err := comments.CommentEdit(ts.Session, id, "@frank"); err != nil {
		t.Fatal(err)
	}
	want("collaborators unknown")
	ts.SetMembers(replicate.Members{Collaborators: []replicate.Collaborator{}})
	if _, err := comments.CommentEdit(ts.Session, id, "@gina"); err != nil {
		t.Fatal(err)
	}
	want("nobody can read", "gina cannot read me")
}

func TestMentionNoMembersFile(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AsAccount("alice")
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "t"}); err != nil {
		t.Fatal(err)
	}
	if _, err := comments.CommentAdd(ts.Session, "ME-1", "@carol"); err != nil {
		t.Fatal(err)
	}
	if len(ts.Warns) != 0 {
		t.Errorf("warnings = %q; want none", ts.Warns)
	}
}

func TestMentionWarningsUseTheStoreWritten(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	members := func(name string, ids ...string) {
		t.Helper()
		s, err := ts.ForStore(name).OpenStore()
		if err != nil {
			t.Fatal(err)
		}
		m := replicate.Members{Collaborators: []replicate.Collaborator{}}
		for _, id := range ids {
			m.Collaborators = append(m.Collaborators, replicate.Collaborator{ID: id, Permission: "write"})
		}
		if err := replicate.WriteMembers(s, m); err != nil {
			t.Fatal(err)
		}
	}
	members("org-a", "github.com/carol")
	members("org-b", "github.com/bob")
	check := func(label string, want ...string) {
		t.Helper()
		if !slices.Equal(ts.Warns, want) {
			t.Errorf("%s: warnings = %q; want %q", label, ts.Warns, want)
		}
		ts.Warns = nil
	}

	// No store is named: the task decides which store is written.
	ts.Warns = nil
	c, err := comments.CommentAdd(ts.Session, "WEB-1", "@bob @carol")
	if err != nil {
		t.Fatal(err)
	}
	check("comment on WEB-1", "carol cannot read org-b")
	if _, err := comments.CommentEdit(ts.Session, c.Ref.Comment.String(), "@bob @carol @dave"); err != nil {
		t.Fatal(err)
	}
	check("comment edit", "dave cannot read org-b")
	if _, err := comments.CommentAdd(ts.Session, "API-1", "@bob @carol"); err != nil {
		t.Fatal(err)
	}
	check("comment on API-1", "bob cannot read org-a")
	es, err := tasks.BeginEdit(ts.Session, "WEB-1")
	if err != nil {
		t.Fatal(err)
	}
	es.Old.Body = "@carol\n"
	b, err := es.Old.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Edit(ts.Session, "WEB-1", b); err != nil {
		t.Fatal(err)
	}
	check("edit of WEB-1", "carol cannot read org-b")
}
