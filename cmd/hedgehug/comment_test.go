package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

func (te *testEnv) commentJSON(args ...string) commentOut {
	te.t.Helper()
	var got commentOut
	out := te.ok(append(args, "--json")...)
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		te.t.Fatalf("%v: %s", err, out)
	}
	return got
}

// comments returns every comment of task id, deleted ones included.
func (te *testEnv) comments(id string) []*model.Comment {
	te.t.Helper()
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		te.t.Fatal(err)
	}
	var out []*model.Comment
	for c, err := range s.Comments(mustID(te.t, id)) {
		if err != nil {
			te.t.Fatal(err)
		}
		out = append(out, c)
	}
	return out
}

func TestCommentAddFromArgs(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	before := len(te.log("me"))

	got := te.commentJSON("comment", x.Short, "hello", "world")
	if got.Body != "hello world\n" || got.Task != x.ID || len(got.Short) < 4 || got.Author != "" || got.DeletedAt != nil {
		t.Errorf("comment = %+v", got)
	}
	if !strings.HasSuffix(got.ID, got.Short) {
		t.Errorf("short %q is not the end of %q", got.Short, got.ID)
	}
	cs := te.comments(x.ID)
	if len(cs) != 1 || cs[0].Body != "hello world\n" {
		t.Fatalf("comments = %+v", cs)
	}
	log := te.log("me")
	if len(log) != before+1 || !strings.Contains(log[0].Message, "Hedgehug-Op: comment") || !strings.Contains(log[0].Message, "Hedgehug-Task: "+x.ID) {
		t.Errorf("commits = %d (want %d), last %q", len(log), before+1, log[0].Message)
	}

	out := te.ok("comment", x.Short, "again")
	if !strings.HasPrefix(out, "Added ") || len(strings.Fields(out)) != 2 {
		t.Errorf("stdout = %q, want Added <short>", out)
	}
}

func TestCommentAddInEditor(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	seen := te.editWith("from the editor\n")

	te.ok("comment", x.Short)
	if got := seen(); len(got) != 1 || got[0] != "" {
		t.Errorf("the editor saw %q, want an empty file", got)
	}
	cs := te.comments(x.ID)
	if len(cs) != 1 || cs[0].Body != "from the editor\n" {
		t.Errorf("comments = %+v", cs)
	}
	if fs := te.tempFiles(); len(fs) != 0 {
		t.Errorf("temporary files left: %v", fs)
	}
}

func TestCommentEmptyBody(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	before := len(te.log("me"))

	code, _, errs := te.run("comment", x.Short, "  ", "\t")
	if code != 1 || !strings.Contains(errs, "empty") {
		t.Errorf("args: exit %d, stderr %q", code, errs)
	}
	te.editWith("  \n\n")
	code, _, errs = te.run("comment", x.Short)
	if code != 1 || !strings.Contains(errs, "empty") {
		t.Errorf("editor: exit %d, stderr %q", code, errs)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
	if fs := te.tempFiles(); len(fs) != 0 {
		t.Errorf("temporary files left: %v", fs)
	}
}

func TestCommentDeletedTask(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	te.ok("rm", x.Short)

	code, _, errs := te.run("comment", x.Short, "hi")
	if code != 1 || !strings.Contains(errs, "restore") {
		t.Errorf("exit %d, stderr %q, want a pointer to restore", code, errs)
	}
}

func TestCommentEdit(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	c := te.commentJSON("comment", x.Short, "first")
	before := len(te.log("me"))
	seen := te.editWith("second\n")

	out := te.ok("comment", "edit", c.Short)
	if got := seen(); len(got) != 1 || got[0] != "first\n" {
		t.Errorf("the editor saw %q, want only the body", got)
	}
	if !strings.HasPrefix(out, "Updated ") {
		t.Errorf("stdout = %q", out)
	}
	cs := te.comments(x.ID)
	if len(cs) != 1 || cs[0].Body != "second\n" || cs[0].ID.String() != c.ID {
		t.Errorf("comments = %+v", cs)
	}
	log := te.log("me")
	if len(log) != before+1 || !strings.Contains(log[0].Message, "Hedgehug-Op: comment") || !strings.Contains(log[0].Message, "Hedgehug-Task: "+x.ID) {
		t.Errorf("last commit %q", log[0].Message)
	}
	if fs := te.tempFiles(); len(fs) != 0 {
		t.Errorf("temporary files left: %v", fs)
	}
}

func TestCommentEditNoChangesAndEmpty(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	c := te.commentJSON("comment", x.Short, "first")
	before := len(te.log("me"))

	te.editWith("first\n")
	if out := te.ok("comment", "edit", c.Short); !strings.HasPrefix(out, "No changes") {
		t.Errorf("stdout = %q, want No changes", out)
	}
	te.editWith(" \n")
	code, _, errs := te.run("comment", "edit", c.Short)
	if code != 1 || !strings.Contains(errs, "comment rm") {
		t.Errorf("exit %d, stderr %q, want a pointer to comment rm", code, errs)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
}

func TestCommentRm(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	c := te.commentJSON("comment", x.Short, "gone soon")
	before := len(te.log("me"))

	out := te.ok("comment", "rm", c.Short)
	if !strings.HasPrefix(out, "Deleted ") {
		t.Errorf("stdout = %q", out)
	}
	cs := te.comments(x.ID)
	if len(cs) != 1 || cs[0].DeletedAt == nil {
		t.Fatalf("comments = %+v, want one with deleted_at", cs)
	}
	log := te.log("me")
	if len(log) != before+1 || !strings.Contains(log[0].Message, "Hedgehug-Op: comment") {
		t.Errorf("last commit %q", log[0].Message)
	}
	if out := te.ok("show", x.Short); strings.Contains(out, "gone soon") || strings.Contains(out, "Comments:") {
		t.Errorf("show still has the comment:\n%s", out)
	}
	if code, _, errs := te.run("comment", "rm", c.Short); code != 1 || !strings.Contains(errs, "deleted") {
		t.Errorf("second rm: exit %d, stderr %q", code, errs)
	}
	te.editWith("x\n")
	if code, _, errs := te.run("comment", "edit", c.Short); code != 1 || !strings.Contains(errs, "deleted") {
		t.Errorf("edit of a deleted comment: exit %d, stderr %q", code, errs)
	}
	if n := len(te.log("me")); n != before+1 {
		t.Errorf("commits = %d, want %d", n, before+1)
	}
}

func TestCommentUnknown(t *testing.T) {
	te := newStoreEnv(t)
	te.addJSON("Alpha")
	if code, _, errs := te.run("comment", "rm", "ZZZZ"); code != 1 || errs == "" {
		t.Errorf("exit %d, stderr %q", code, errs)
	}
}

// putComments writes comments with the given IDs on task id.
func (te *testEnv) putComments(id string, cids ...string) {
	te.t.Helper()
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		te.t.Fatal(err)
	}
	if _, err := s.Update(store.Op{Name: "comment", Author: gitrepo.Signature{Name: "T", Email: "t@example.com"}}, func(tx *store.Tx) error {
		for _, cid := range cids {
			if err := tx.PutComment(mustID(te.t, id), &model.Comment{ID: mustID(te.t, cid), Body: "body " + cid + "\n"}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		te.t.Fatal(err)
	}
}

func TestCommentShortIDs(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	y := te.addJSON("Beta")
	const a, b = "01HZZZZZZZZZZZZZZZZZZZ0001", "01HYYYYYYYYYYYYYYYYYYY0001"
	te.putComments(x.ID, a)
	te.putComments(y.ID, b)

	code, _, errs := te.run("comment", "rm", "0001")
	if code != 1 || !strings.Contains(errs, "matches 2") || !strings.Contains(errs, "Z0001  Alpha") || !strings.Contains(errs, "Y0001  Beta") {
		t.Errorf("exit %d, stderr %q, want both candidates with the titles", code, errs)
	}
	if out := te.ok("show", x.Short); !strings.Contains(out, "Z0001") {
		t.Errorf("show has no short ID Z0001:\n%s", out)
	}
	te.ok("comment", "rm", "z0001")
	cs := te.comments(x.ID)
	if len(cs) != 1 || cs[0].DeletedAt == nil {
		t.Errorf("comments = %+v, want the first deleted", cs)
	}
	if cs := te.comments(y.ID); cs[0].DeletedAt != nil {
		t.Errorf("the other comment was deleted")
	}
	// A full ULID resolves too, in any case.
	te.ok("comment", "rm", strings.ToLower(b))
}

func TestShowCommentShort(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	c := te.commentJSON("comment", x.Short, "has an author-less comment")

	out := te.ok("show", x.Short)
	if !strings.Contains(out, "\n  "+c.Short+"  -  2026-") {
		t.Errorf("show does not print the short ID and - for the author:\n%s", out)
	}
	var got struct {
		Comments []struct{ ID, Short string } `json:"comments"`
	}
	if err := json.Unmarshal([]byte(te.ok("show", x.Short, "--json")), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Comments) != 1 || got.Comments[0].Short != c.Short || got.Comments[0].ID != c.ID {
		t.Errorf("comments = %+v, want short %q", got.Comments, c.Short)
	}
}
