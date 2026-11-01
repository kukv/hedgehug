package ops

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// asAccount makes login the only account of config.yaml, which has its
// accounts as the last key.
func (ts *testSession) asAccount(login string) {
	ts.t.Helper()
	_, p, err := ts.Config()
	if err != nil {
		ts.t.Fatal(err)
	}
	b, err := os.ReadFile(p.ConfigFile)
	if err != nil {
		ts.t.Fatal(err)
	}
	text, _, _ := strings.Cut("\n"+string(b), "\naccounts:")
	text = strings.TrimPrefix(text, "\n")
	if text != "" {
		text += "\n"
	}
	text += "accounts:\n  - {name: " + login + ", host: github.com, login: " + login + "}\n"
	if err := os.WriteFile(p.ConfigFile, []byte(text), 0o600); err != nil {
		ts.t.Fatal(err)
	}
	ts.resetWriter()
}

func (ts *testSession) setMembers(m sync.Members) {
	ts.t.Helper()
	s, _, err := ts.Open()
	if err != nil {
		ts.t.Fatal(err)
	}
	if err := sync.WriteMembers(s, m); err != nil {
		ts.t.Fatal(err)
	}
}

func TestCheckCommentEditable(t *testing.T) {
	ref := store.CommentRef{}
	me := []string{"github.com/Alice"}
	c := &model.Comment{ULID: model.NewULID(), Author: "github.com/alice"}
	if err := CheckCommentEditable(ref, c, me, "edited"); err != nil {
		t.Errorf("own comment: %v", err)
	}
	c.Author = "github.com/bob"
	err := CheckCommentEditable(ref, c, me, "removed")
	wantErr(t, err, KindConflict, "comment.not_author")
	if want := "github.com/bob's comment cannot be removed"; err != nil && err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
	c.Author = ""
	err = CheckCommentEditable(ref, c, me, "edited")
	wantErr(t, err, KindConflict, "comment.no_author")
	if want := "a comment whose author is unknown cannot be edited"; err != nil && err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
	c.Author = "github.com/bob"
	now := time.Now()
	c.DeletedAt = &now
	wantErr(t, CheckCommentEditable(ref, c, me, "edited"), KindConflict, "comment.deleted")
}

func TestCommentAuthorOps(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	ts.asAccount("alice")
	c, err := ts.CommentAdd("ME-1", "mine")
	if err != nil || c.Comment.Author != "github.com/alice" {
		t.Fatalf("CommentAdd = %+v, %v", c.Comment, err)
	}
	id := c.Ref.Comment.String()
	if _, err := ts.CommentEdit(id, "edited"); err != nil {
		t.Errorf("own edit: %v", err)
	}
	ts.asAccount("bob")
	_, err = ts.CommentEdit(id, "hijack")
	wantErr(t, err, KindConflict, "comment.not_author")
	_, err = ts.CommentRemove(id)
	wantErr(t, err, KindConflict, "comment.not_author")
	ts.asAccount("Alice")
	if _, err := ts.CommentRemove(id); err != nil {
		t.Errorf("own rm: %v", err)
	}
	// Deleted and someone else's: the deleted error comes first.
	ts.asAccount("bob")
	_, err = ts.CommentRemove(id)
	wantErr(t, err, KindConflict, "comment.deleted")
	_, err = ts.CommentEdit(id, "x")
	wantErr(t, err, KindConflict, "comment.deleted")
}

func TestCommentWithoutAuthor(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	c, err := ts.CommentAdd("ME-1", "nobody")
	if err != nil || c.Comment.Author != "" {
		t.Fatalf("CommentAdd = %+v, %v", c.Comment, err)
	}
	ts.asAccount("alice")
	_, err = ts.CommentEdit(c.Ref.Comment.String(), "x")
	wantErr(t, err, KindConflict, "comment.no_author")
	_, err = ts.CommentRemove(c.Ref.Comment.String())
	wantErr(t, err, KindConflict, "comment.no_author")
}

func TestMentionWarnings(t *testing.T) {
	ts := newTestSession(t)
	ts.asAccount("alice")
	ts.setMembers(sync.Members{Collaborators: []sync.Collaborator{{ID: "github.com/bob", Permission: "write"}}})
	if _, err := ts.Add(AddInput{Title: "t"}); err != nil {
		t.Fatal(err)
	}
	edit := func(body string) {
		t.Helper()
		es, err := ts.BeginEdit("ME-1")
		if err != nil {
			t.Fatal(err)
		}
		es.Old.Body = body
		b, err := es.Old.Encode()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ts.Edit("ME-1", b); err != nil {
			t.Fatal(err)
		}
	}
	want := func(label string, ws ...string) {
		t.Helper()
		if !slices.Equal(ts.warns, ws) {
			t.Errorf("%s: warnings = %q; want %q", label, ts.warns, ws)
		}
		ts.warns = nil
	}
	edit("hi @bob and @carol\n")
	want("task edit", "carol cannot read me")
	edit("hi @bob and @carol\nmore text\n")
	want("carol was there")
	edit("hi @github.com/carol @carol @Github.com/Bob\n")
	want("carol was there, bob reads")
	edit("@github.com/dave @dave\n")
	want("the same login once", "dave cannot read me")

	c, err := ts.CommentAdd("ME-1", "ping @bob @carol")
	if err != nil {
		t.Fatal(err)
	}
	want("comment add", "carol cannot read me")
	id := c.Ref.Comment.String()
	if _, err := ts.CommentEdit(id, "ping @bob @carol\nand @erin"); err != nil {
		t.Fatal(err)
	}
	want("comment edit", "erin cannot read me")

	ts.setMembers(sync.Members{})
	if _, err := ts.CommentEdit(id, "@frank"); err != nil {
		t.Fatal(err)
	}
	want("collaborators unknown")
	ts.setMembers(sync.Members{Collaborators: []sync.Collaborator{}})
	if _, err := ts.CommentEdit(id, "@gina"); err != nil {
		t.Fatal(err)
	}
	want("nobody can read", "gina cannot read me")
}

func TestMentionNoMembersFile(t *testing.T) {
	ts := newTestSession(t)
	ts.asAccount("alice")
	if _, err := ts.Add(AddInput{Title: "t"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.CommentAdd("ME-1", "@carol"); err != nil {
		t.Fatal(err)
	}
	if len(ts.warns) != 0 {
		t.Errorf("warnings = %q; want none", ts.warns)
	}
}

func TestMentionWarningsUseTheStoreWritten(t *testing.T) {
	ts := newTwoStoreSession(t)
	members := func(name string, ids ...string) {
		t.Helper()
		s, err := ts.ForStore(name).OpenStore()
		if err != nil {
			t.Fatal(err)
		}
		m := sync.Members{Collaborators: []sync.Collaborator{}}
		for _, id := range ids {
			m.Collaborators = append(m.Collaborators, sync.Collaborator{ID: id, Permission: "write"})
		}
		if err := sync.WriteMembers(s, m); err != nil {
			t.Fatal(err)
		}
	}
	members("org-a", "github.com/carol")
	members("org-b", "github.com/bob")
	check := func(label string, want ...string) {
		t.Helper()
		if !slices.Equal(ts.warns, want) {
			t.Errorf("%s: warnings = %q; want %q", label, ts.warns, want)
		}
		ts.warns = nil
	}

	// No store is named: the task decides which store is written.
	ts.warns = nil
	c, err := ts.CommentAdd("WEB-1", "@bob @carol")
	if err != nil {
		t.Fatal(err)
	}
	check("comment on WEB-1", "carol cannot read org-b")
	if _, err := ts.CommentEdit(c.Ref.Comment.String(), "@bob @carol @dave"); err != nil {
		t.Fatal(err)
	}
	check("comment edit", "dave cannot read org-b")
	if _, err := ts.CommentAdd("API-1", "@bob @carol"); err != nil {
		t.Fatal(err)
	}
	check("comment on API-1", "bob cannot read org-a")
	es, err := ts.BeginEdit("WEB-1")
	if err != nil {
		t.Fatal(err)
	}
	es.Old.Body = "@carol\n"
	b, err := es.Old.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Edit("WEB-1", b); err != nil {
		t.Fatal(err)
	}
	check("edit of WEB-1", "carol cannot read org-b")
}
