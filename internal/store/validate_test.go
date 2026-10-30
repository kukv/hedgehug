package store

import (
	"errors"
	"os"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

func TestUpdateRefusesViolation(t *testing.T) {
	s := newTestStore(t)
	task := newTask("")
	err := s.Update(Op{Name: "add", Task: task.ID, Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(task) })
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Task != task.ID || len(ve.Violations) != 1 || ve.Violations[0].Code != schema.CodeMissingTitle {
		t.Fatalf("err = %v, want a ValidationError for the empty title", err)
	}
	if _, err := os.Stat(s.path(model.TaskPath(task.ID))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("refused task was written: %v", err)
	}
	if n := len(headLog(t, s.dir)); n != 1 {
		t.Errorf("commits = %d, want 1", n)
	}
}

func TestUpdateAllowsOldViolation(t *testing.T) {
	s := newTestStore(t)
	task := newTask("x")
	addTask(t, s, task)
	// A sync may bring a kind this schema does not know.
	mustWrite(t, s, model.TaskPath(task.ID), "---\nid: "+task.ID.String()+"\nkind: nope\ntitle: x\nstatus: todo\n---\n")

	got, err := s.Task(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	got.Title = "renamed"
	if err := s.Update(Op{Name: "update", Task: task.ID, Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(got) }); err != nil {
		t.Errorf("editing a task with an old violation failed: %v", err)
	}
}

func TestUpdateRemoveThenPutIsValidated(t *testing.T) {
	s := newTestStore(t)
	task := newTask("x")
	addTask(t, s, task)

	bad := newTask("")
	bad.ID = task.ID
	err := s.Update(Op{Name: "update", Task: task.ID, Author: testAuthor}, func(tx *Tx) error {
		return errors.Join(tx.RemoveTask(task.ID), tx.PutTask(bad))
	})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Violations[0].Code != schema.CodeMissingTitle {
		t.Errorf("err = %v, want a ValidationError for the empty title", err)
	}
	if n := len(headLog(t, s.dir)); n != 2 {
		t.Errorf("commits = %d, want 2", n)
	}
}

func TestUpdatePlacement(t *testing.T) {
	s := newTestStore(t)
	parent := newTask("parent")
	child := newTask("child")
	child.Kind = "bug" // a bug cannot sit under a task
	child.Parent = &parent.ID
	err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error {
		return errors.Join(tx.PutTask(parent), tx.PutTask(child))
	})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Task != child.ID || ve.Violations[0].Code != schema.CodePlacement {
		t.Errorf("err = %v, want a placement violation on the child", err)
	}
}

func TestUpdateParentAndChildTogether(t *testing.T) {
	s := newTestStore(t)
	parent := newTask("parent")
	parent.Kind = "epic"
	child := newTask("child")
	child.Parent = &parent.ID
	if err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error {
		// The child is put first: the check runs after fn, when both are queued.
		return errors.Join(tx.PutTask(child), tx.PutTask(parent))
	}); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateMissingParent(t *testing.T) {
	s := newTestStore(t)
	child := newTask("child")
	missing := model.NewID()
	child.Parent = &missing
	err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(child) })
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateParentCycle(t *testing.T) {
	s := newTestStore(t)
	a, b := newTask("a"), newTask("b")
	a.Parent, b.Parent = &b.ID, &a.ID
	err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error {
		return errors.Join(tx.PutTask(a), tx.PutTask(b))
	})
	if !errors.Is(err, ErrCycle) {
		t.Errorf("err = %v, want ErrCycle", err)
	}
}

func TestUpdateUsesProjectSchema(t *testing.T) {
	s := newTestStore(t)
	project := newTask("project")
	project.Kind = "project"
	addTask(t, s, project)

	sc := schema.Default()
	sc.Scoped = append(sc.Scoped, schema.Scoped{
		Where:    "project:" + project.ID.String(),
		Workflow: &schema.Workflow{Required: schema.Lists{{Key: "in_progress", Values: []string{"assignees"}}}},
	})
	if err := s.Update(Op{Name: "schema", Author: testAuthor}, func(tx *Tx) error { return tx.PutSchema(sc) }); err != nil {
		t.Fatal(err)
	}

	inside := newTask("inside")
	inside.Parent = &project.ID
	inside.Status = "in_progress"
	err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(inside) })
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Violations[0].Code != schema.CodeRequired {
		t.Errorf("inside the project: err = %v, want a required violation", err)
	}

	outside := newTask("outside")
	outside.Status = "in_progress"
	if err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(outside) }); err != nil {
		t.Errorf("outside the project: %v", err)
	}
}
