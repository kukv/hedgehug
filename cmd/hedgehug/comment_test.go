package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

	got := te.commentJSON("comment", x.Key, "hello", "world")
	if got.Body != "hello world\n" || got.Task != x.ID || len(got.ID) != 26 || got.Author != "" || got.DeletedAt != nil {
		t.Errorf("comment = %+v", got)
	}
	cs := te.comments(x.ID)
	if len(cs) != 1 || cs[0].Body != "hello world\n" {
		t.Fatalf("comments = %+v", cs)
	}
	log := te.log("me")
	if len(log) != before+1 || !strings.Contains(log[0].Message, "Hedgehug-Op: comment") || !strings.Contains(log[0].Message, "Hedgehug-Task: "+x.ID) {
		t.Errorf("commits = %d (want %d), last %q", len(log), before+1, log[0].Message)
	}

	out := te.ok("comment", x.Key, "again")
	if f := strings.Fields(out); len(f) != 2 || f[0] != "Added" || len(f[1]) != 26 {
		t.Errorf("stdout = %q, want Added <ULID>", out)
	}
}

func TestCommentAddInEditor(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	seen := te.editWith("from the editor\n")

	te.ok("comment", x.Key)
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

// When the comment cannot be written, the text typed in the editor is kept
// in its file, which the error names.
func TestCommentAddKeepsFileOnFailure(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	before := len(te.log("me"))
	// A file named comments makes the directory of the comment impossible.
	te.editWith("from the editor\nTOUCH " + filepath.Join(te.storeDir("me"), "comments") + "\n")

	code, _, stderr := te.run("comment", x.Key)
	if code != 1 {
		t.Errorf("exit %d (want 1), stderr %q", code, stderr)
	}
	fs := te.tempFiles()
	if len(fs) != 1 || !strings.Contains(stderr, fs[0]) || !strings.HasPrefix(filepath.Base(fs[0]), "hedgehug-"+x.Key+"-") {
		t.Fatalf("temporary files %v, stderr %q: want one named after the key, named in stderr", fs, stderr)
	}
	if b, err := os.ReadFile(fs[0]); err != nil || string(b) != "from the editor\n" {
		t.Errorf("kept file = %q, %v", b, err)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
}

func TestCommentEmptyBody(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	before := len(te.log("me"))

	code, _, errs := te.run("comment", x.Key, "  ", "\t")
	if code != 1 || !strings.Contains(errs, "empty") {
		t.Errorf("args: exit %d, stderr %q", code, errs)
	}
	te.editWith("  \n\n")
	code, _, errs = te.run("comment", x.Key)
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
	te.ok("rm", x.Key)

	code, _, errs := te.run("comment", x.Key, "hi")
	if code != 1 || !strings.Contains(errs, "restore") {
		t.Errorf("exit %d, stderr %q, want a pointer to restore", code, errs)
	}
}

func TestCommentEdit(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	c := te.commentJSON("comment", x.Key, "first")
	before := len(te.log("me"))
	seen := te.editWith("second\n")

	out := te.ok("comment", "edit", c.ID)
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

func TestCommentEditChangedWhileEditing(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	c := te.commentJSON("comment", x.Key, "first")
	before := len(te.log("me"))
	file := filepath.Join(te.storeDir("me"), filepath.FromSlash(model.CommentPath(mustID(t, x.ID), mustID(t, c.ID))))
	te.editWith("second\nTOUCH " + file + "\n")

	code, _, stderr := te.run("comment", "edit", c.ID)
	if code != 1 || !strings.Contains(stderr, "changed while editing") {
		t.Errorf("exit %d (want 1), stderr %q", code, stderr)
	}
	fs := te.tempFiles()
	if len(fs) != 1 || !strings.Contains(stderr, fs[0]) || !strings.HasPrefix(filepath.Base(fs[0]), "hedgehug-comment-"+c.ID+"-") {
		t.Fatalf("temporary files %v, stderr %q: want one named after the comment, named in stderr", fs, stderr)
	}
	if b, err := os.ReadFile(fs[0]); err != nil || string(b) != "second\n" {
		t.Errorf("kept file = %q, %v", b, err)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
}

func TestCommentEditNoChangesAndEmpty(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	c := te.commentJSON("comment", x.Key, "first")
	before := len(te.log("me"))

	te.editWith("first\n")
	if out := te.ok("comment", "edit", c.ID); !strings.HasPrefix(out, "No changes") {
		t.Errorf("stdout = %q, want No changes", out)
	}
	te.editWith(" \n")
	code, _, errs := te.run("comment", "edit", c.ID)
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
	c := te.commentJSON("comment", x.Key, "gone soon")
	before := len(te.log("me"))

	out := te.ok("comment", "rm", c.ID)
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
	if out := te.ok("show", x.Key); strings.Contains(out, "gone soon") || strings.Contains(out, "Comments:") {
		t.Errorf("show still has the comment:\n%s", out)
	}
	if code, _, errs := te.run("comment", "rm", c.ID); code != 1 || !strings.Contains(errs, "deleted") {
		t.Errorf("second rm: exit %d, stderr %q", code, errs)
	}
	te.editWith("x\n")
	if code, _, errs := te.run("comment", "edit", c.ID); code != 1 || !strings.Contains(errs, "deleted") {
		t.Errorf("edit of a deleted comment: exit %d, stderr %q", code, errs)
	}
	if n := len(te.log("me")); n != before+1 {
		t.Errorf("commits = %d, want %d", n, before+1)
	}
}

// comment rm writes the comment as it is under the lock: an edit that
// lands between its first read and the lock is kept.
func TestCommentRmRereadsUnderLock(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	c := te.commentJSON("comment", x.Key, "first")
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		t.Fatal(err)
	}
	task := mustID(t, x.ID)
	locked, proceed := make(chan struct{}), make(chan struct{})
	edited := make(chan error, 1)
	go func() {
		author := gitrepo.Signature{Name: "Bob", Email: "bob@example.com"}
		_, err := s.Update(store.Op{Name: "comment", Author: author}, func(tx *store.Tx) error {
			close(locked)
			<-proceed
			for cur, err := range s.Comments(task) {
				if err != nil {
					return err
				}
				cur.Body = "second\n"
				return tx.PutComment(task, cur)
			}
			return nil
		})
		edited <- err
	}()
	<-locked
	type result struct {
		code   int
		stderr string
	}
	rm := make(chan result, 1)
	go func() {
		code, _, stderr := te.run("comment", "rm", c.ID)
		rm <- result{code, stderr}
	}()
	// Let comment rm read the comment and wait for the lock.
	time.Sleep(300 * time.Millisecond)
	close(proceed)
	if err := <-edited; err != nil {
		t.Fatal(err)
	}
	if r := <-rm; r.code != 0 {
		t.Fatalf("comment rm: exit %d, stderr %q", r.code, r.stderr)
	}
	cs := te.comments(x.ID)
	if len(cs) != 1 || cs[0].Body != "second\n" || cs[0].DeletedAt == nil {
		t.Errorf("comments = %+v, want the edited body, deleted", cs)
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

func TestCommentIDMustBeFull(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	const a = "01HZZZZZZZZZZZZZZZZZZZ0001"
	te.putComments(x.ID, a)

	for _, in := range []string{"0001", a[1:]} {
		if code, _, errs := te.run("comment", "rm", in); code != 1 || !strings.Contains(errs, "give the full ID of the comment") || !strings.Contains(errs, "not found") {
			t.Errorf("%s: exit %d, stderr %q, want the full ID asked for", in, code, errs)
		}
	}
	// A full ULID resolves, in any case.
	te.ok("comment", "rm", strings.ToLower(a))
}

func TestShowCommentID(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	c := te.commentJSON("comment", x.Key, "has an author-less comment")

	out := te.ok("show", x.Key)
	if !strings.Contains(out, "\n  "+c.ID+"  -  2026-") {
		t.Errorf("show does not print the ID and - for the author:\n%s", out)
	}
	var got struct {
		Comments []map[string]any `json:"comments"`
	}
	if err := json.Unmarshal([]byte(te.ok("show", x.Key, "--json")), &got); err != nil {
		t.Fatal(err)
	}
	// id, author, created and body.
	if len(got.Comments) != 1 || got.Comments[0]["id"] != c.ID || len(got.Comments[0]) != 4 {
		t.Errorf("comments = %+v, want only id %q, author, created and body", got.Comments, c.ID)
	}
}
