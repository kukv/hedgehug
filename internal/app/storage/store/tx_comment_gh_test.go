package store

import (
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
)

func readComment(t *testing.T, s *Store, task, comment model.ULID) *model.Comment {
	t.Helper()
	for c, err := range s.Comments(task) {
		if err != nil {
			t.Fatal(err)
		}
		if c.ULID == comment {
			return c
		}
	}
	t.Fatalf("comment %s not found", comment)
	return nil
}

func TestPutCommentGHKeepsUpdated(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	task := newTask("x")
	addTask(t, s, task)
	c := &model.Comment{ULID: model.NewULID(), Author: "github.com/me", Body: "hello\n"}
	if _, err := s.Update(Op{Name: "comment", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error { return tx.PutComment(task.ULID, c) }); err != nil {
		t.Fatal(err)
	}

	later := testNow.Add(time.Hour)
	s.now = func() time.Time { return later }
	got := readComment(t, s, task.ULID, c.ULID)
	if err := got.SetGitHub([]model.CommentLink{{Ref: "github.com/o/r/issues/1", ID: 7, BodySynced: "hello"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(Op{Name: "github-push", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error { return tx.PutCommentGH(task.ULID, got) }); err != nil {
		t.Fatal(err)
	}
	after := readComment(t, s, task.ULID, c.ULID)
	l, err := after.GitHub()
	if err != nil || len(l) != 1 || l[0].ID != 7 {
		t.Fatalf("gh = %+v, %v", l, err)
	}
	if !after.Updated.Equal(testNow) {
		t.Errorf("updated = %v, want %v: a write of gh must not advance it", after.Updated, testNow)
	}

	// A write of the body advances it, and gh stays.
	after.Body = "edited\n"
	if _, err := s.Update(Op{Name: "comment-edit", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error { return tx.PutComment(task.ULID, after) }); err != nil {
		t.Fatal(err)
	}
	edited := readComment(t, s, task.ULID, c.ULID)
	if !edited.Updated.Equal(later) {
		t.Errorf("updated = %v, want %v after an edit", edited.Updated, later)
	}
	if l, err := edited.GitHub(); err != nil || len(l) != 1 {
		t.Errorf("gh after the edit = %+v, %v", l, err)
	}
}

func TestPutCommentGHRefusesOtherChanges(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	task := newTask("x")
	addTask(t, s, task)
	c := &model.Comment{ULID: model.NewULID(), Author: "github.com/me", Body: "hello\n"}
	if _, err := s.Update(Op{Name: "comment", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error { return tx.PutComment(task.ULID, c) }); err != nil {
		t.Fatal(err)
	}
	put := func(mod func(*model.Comment)) error {
		got := readComment(t, s, task.ULID, c.ULID)
		mod(got)
		_, err := s.Update(Op{Name: "github-push", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error { return tx.PutCommentGH(task.ULID, got) })
		return err
	}
	if err := put(func(c *model.Comment) { c.Body = "other\n" }); !errors.Is(err, ErrCommentChanged) {
		t.Errorf("a changed body: %v", err)
	}
	if err := put(func(c *model.Comment) { n := testNow; c.DeletedAt = &n }); !errors.Is(err, ErrCommentChanged) {
		t.Errorf("a changed deleted_at: %v", err)
	}
	if err := put(func(c *model.Comment) { c.Updated = c.Updated.Add(time.Second) }); !errors.Is(err, ErrCommentChanged) {
		t.Errorf("a changed updated: %v", err)
	}
	// A comment that is not in the file.
	missing := &model.Comment{ULID: model.NewULID(), Author: "github.com/me", Body: "x\n"}
	if _, err := s.Update(Op{Name: "github-push", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error { return tx.PutCommentGH(task.ULID, missing) }); !errors.Is(err, ErrNotFound) {
		t.Errorf("a comment with no file: %v", err)
	}
	// One written earlier in the same operation.
	if _, err := s.Update(Op{Name: "github-push", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error {
		got := readComment(t, s, task.ULID, c.ULID)
		got.Body = "again\n"
		if err := tx.PutComment(task.ULID, got); err != nil {
			return err
		}
		return tx.PutCommentGH(task.ULID, got)
	}); !errors.Is(err, ErrCommentChanged) {
		t.Errorf("a comment written in the operation: %v", err)
	}
}
