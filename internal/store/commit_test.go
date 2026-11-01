package store

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

func TestUpdateReportsCommitted(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	task := newTask("x")
	committed, err := s.Update(Op{Name: "add", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(task) })
	if err != nil || !committed {
		t.Fatalf("add: committed, err = %v, %v; want true, nil", committed, err)
	}
	same, err := s.Task(task.ULID)
	if err != nil {
		t.Fatal(err)
	}
	committed, err = s.Update(Op{Name: "update", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(same) })
	if err != nil || committed {
		t.Errorf("same content: committed, err = %v, %v; want false, nil", committed, err)
	}
}

func TestUpdateCommitReturnsTheNewHead(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	task := newTask("x")
	h, err := s.UpdateCommit(Op{Name: "add", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(task) })
	if err != nil {
		t.Fatal(err)
	}
	head, err := s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	if h.IsZero() || h != head {
		t.Errorf("UpdateCommit = %s; want HEAD %s", h, head)
	}
	same, err := s.Task(task.ULID)
	if err != nil {
		t.Fatal(err)
	}
	h, err = s.UpdateCommit(Op{Name: "update", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(same) })
	if err != nil || !h.IsZero() {
		t.Errorf("same content: hash, err = %s, %v; want the zero hash, nil", h, err)
	}
	if after, _ := s.Repo().Head(); after != head {
		t.Errorf("HEAD moved to %s without a commit", after)
	}
	boom := errors.New("boom")
	if h, err = s.UpdateCommit(Op{Name: "update", Author: testAuthor}, func(*Tx) error { return boom }); !errors.Is(err, boom) || !h.IsZero() {
		t.Errorf("failed operation: hash, err = %s, %v; want the zero hash, boom", h, err)
	}
}

func TestUpdateSameAsHeadWithDirtyWorkTree(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	task := newTask("x")
	addTask(t, s, task)
	rel := model.TaskPath(task.ULID)
	want, err := os.ReadFile(s.path(rel))
	if err != nil {
		t.Fatal(err)
	}
	// The work tree differs from HEAD, uncommitted, as after a hand edit.
	mustWrite(t, s, rel, string(want)+"\nhand edit\n")
	before := len(headLog(t, s.dir))

	same, err := model.DecodeTask(want)
	if err != nil {
		t.Fatal(err)
	}
	committed, err := s.Update(Op{Name: "update", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(same) })
	if err != nil || committed {
		t.Fatalf("committed, err = %v, %v; want false, nil", committed, err)
	}
	if n := len(headLog(t, s.dir)); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
	got, err := os.ReadFile(s.path(rel))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("file = %q, want HEAD's %q", got, want)
	}
}

func TestRemoveTaskDropsQueuedChildren(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	task := newTask("x")
	addTask(t, s, task)
	c := &model.Comment{ULID: model.NewULID(), Author: "github.com/me"}
	if _, err := s.Update(Op{Name: "purge", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error {
		return errors.Join(tx.PutComment(task.ULID, c), tx.RemoveTask(task.ULID))
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.path(model.CommentPath(task.ULID, c.ULID))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the comment file was left: %v", err)
	}
	tree, err := headLog(t, s.dir)[0].Tree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tree.File(model.CommentPath(task.ULID, c.ULID)); err == nil {
		t.Error("the comment is in the commit")
	}
}

func TestPutChildOfRemovedTask(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	task := newTask("x")
	addTask(t, s, task)
	_, err := s.Update(Op{Name: "purge", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error {
		if err := tx.RemoveTask(task.ULID); err != nil {
			return err
		}
		if err := tx.PutComment(task.ULID, &model.Comment{ULID: model.NewULID(), Author: "github.com/me"}); err == nil {
			t.Error("PutComment after RemoveTask succeeded")
		}
		if err := tx.PutWorklog(task.ULID, &model.Worklog{ULID: model.NewULID(), Author: "github.com/me"}); err == nil {
			t.Error("PutWorklog after RemoveTask succeeded")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func forbidTodoToDone() *schema.Schema {
	sc := schema.Default()
	sc.Workflow.Transitions = schema.Lists{{Key: "todo", Values: []string{"in_progress"}}}
	sc.Workflow.Required = schema.Lists{{Key: "done", Values: []string{"assignees"}}}
	return sc
}

func TestUpdateSkipTransitions(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	task := newTask("x")
	task.Assignees = []string{"github.com/me"} // so that a write can empty them
	addTask(t, s, task)
	sc := forbidTodoToDone()
	if _, err := s.Update(Op{Name: "schema", Author: testAuthor}, func(tx *Tx) error { return tx.PutSchema(sc) }); err != nil {
		t.Fatal(err)
	}
	move := func(skip bool, assignees []string) error {
		_, err := s.Update(Op{Name: "update", Task: task.ULID, Author: testAuthor, SkipTransitions: skip}, func(tx *Tx) error {
			t, err := tx.Task(task.ULID)
			if err != nil {
				return err
			}
			t.Status = "done"
			t.Assignees = assignees
			return tx.PutTask(t)
		})
		return err
	}
	code := func(err error) string {
		var ve *ValidationError
		if !errors.As(err, &ve) {
			return ""
		}
		return ve.Violations[0].Code
	}

	if err := move(false, []string{"github.com/me"}); code(err) != schema.CodeTransition {
		t.Errorf("without skip: err = %v, want a transition violation", err)
	}
	if err := move(true, nil); code(err) != schema.CodeRequired {
		t.Errorf("skip emptying a required field: err = %v, want a required violation", err)
	}
	if err := move(true, []string{"github.com/me"}); err != nil {
		t.Errorf("skip: %v", err)
	}
}

func TestTaskBytes(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	task := newTask("x")
	addTask(t, s, task)
	broken := "---\n: :\n---\n"
	mustWrite(t, s, model.TaskPath(task.ULID), broken)
	got, err := s.TaskBytes(task.ULID)
	if err != nil || string(got) != broken {
		t.Errorf("TaskBytes = %q, %v; want %q", got, err, broken)
	}
	if _, err := s.TaskBytes(model.NewULID()); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing: err = %v, want ErrNotFound", err)
	}
}

func TestCommentULIDs(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	a, b := newTask("a"), newTask("b")
	refs := []CommentRef{
		{a.ULID, model.NewULID()}, {a.ULID, model.NewULID()}, {b.ULID, model.NewULID()},
	}
	if _, err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error {
		if err := errors.Join(tx.PutTask(a), tx.PutTask(b)); err != nil {
			return err
		}
		for _, r := range refs {
			if err := tx.PutComment(r.Task, &model.Comment{ULID: r.Comment, Author: "github.com/me"}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, s, "comments/x/y/zzz/abc.md", "x")

	got, err := s.CommentULIDs()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(refs) {
		t.Fatalf("CommentIDs = %v, want %d refs", got, len(refs))
	}
	have := map[CommentRef]bool{}
	for i, r := range got {
		have[r] = true
		if i > 0 && model.CommentPath(got[i-1].Task, got[i-1].Comment) >= model.CommentPath(r.Task, r.Comment) {
			t.Errorf("not in path order: %v", got)
		}
	}
	for _, r := range refs {
		if !have[r] {
			t.Errorf("missing %v", r)
		}
	}
}

func TestAuthorTrailer(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	x := newTask("x")
	if _, err := s.Update(Op{Name: "add", Task: x.ULID, Author: testAuthor, Account: "github.com/alice"}, func(tx *Tx) error { return tx.PutTask(x) }); err != nil {
		t.Fatal(err)
	}
	if msg := headLog(t, s.dir)[0].Message; !strings.Contains(msg, "\nHedgehug-Author: github.com/alice") {
		t.Errorf("message = %q", msg)
	}
	y := newTask("y")
	if _, err := s.Update(Op{Name: "add", Task: y.ULID, Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(y) }); err != nil {
		t.Fatal(err)
	}
	if msg := headLog(t, s.dir)[0].Message; strings.Contains(msg, "Hedgehug-Author") {
		t.Errorf("message = %q", msg)
	}
}
