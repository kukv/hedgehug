package store

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// A task is not put over a forward by accident: a stale write for a task that
// moved to another store would make the task live in two stores.
func TestPutTaskOverAForwardIsRefused(t *testing.T) {
	s := newTestStore(t)
	x := newTask("x")
	if _, err := s.Update(Op{Name: "add", Task: x.ULID, Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(x) }); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, s, model.TaskPath(x.ULID), forwardFile(x.ULID.String()))

	_, err := s.Update(Op{Name: "update", Task: x.ULID, Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(x) })
	var ve *ValidationError
	if !errors.As(err, &ve) || len(ve.Violations) != 1 || ve.Violations[0].Code != CodeForward {
		t.Fatalf("PutTask over a forward: err = %v, want a ValidationError with %q", err, CodeForward)
	}
	want := schema.Violation{Code: CodeForward, Field: "ulid", Msg: "task " + x.ULID.String() + " moved to another store"}
	if ve.Violations[0] != want {
		t.Errorf("violation = %+v, want %+v", ve.Violations[0], want)
	}
	if _, err := s.Task(x.ULID); !errors.Is(err, model.ErrForward) {
		t.Errorf("the forward was replaced: Task err = %v", err)
	}
}
