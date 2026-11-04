package tasks_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// headOp is the Hedgehug-Op of the commit at HEAD of the test store.
func headOp(t *testing.T, ts *usecasetest.Session) string {
	t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	log, err := usecasetest.LogOf(s.Repo(), 1)
	if err != nil {
		t.Fatal(err)
	}
	op, _ := log[0].Trailer("Hedgehug-Op")
	return op
}

func TestPatchSetsFieldsAndBodyInOneCommit(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	before, err := usecasetest.LogOf(s.Repo(), 0)
	if err != nil {
		t.Fatal(err)
	}
	body := "new body\n"
	res, err := tasks.Patch(ts.Session, "ME-1", tasks.PatchInput{
		Assigns: []usecase.Assign{{Key: "title", Op: "=", Value: "b"}, {Key: "status", Op: "=", Value: "in_progress"}},
		Body:    &body,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Committed || res.Commit == "" {
		t.Fatalf("result = %+v, want a commit", res)
	}
	after, err := usecasetest.LogOf(s.Repo(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before)+1 {
		t.Fatalf("commits: %d → %d, want one more", len(before), len(after))
	}
	if op := headOp(t, ts); op != "set" {
		t.Errorf("op = %q, want set", op)
	}
	got, err := s.Task(res.ULID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "b" || got.Status != "in_progress" || got.Body != body {
		t.Errorf("task = %q %q %q", got.Title, got.Status, got.Body)
	}
}

func TestPatchBodyOnlyIsEdit(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	body := "only the body\n"
	if _, err := tasks.Patch(ts.Session, "ME-1", tasks.PatchInput{Body: &body}); err != nil {
		t.Fatal(err)
	}
	if op := headOp(t, ts); op != "edit" {
		t.Errorf("op = %q, want edit", op)
	}
}

func TestPatchNothingIsUsage(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	_, err := tasks.Patch(ts.Session, "ME-1", tasks.PatchInput{})
	usecasetest.WantErr(t, err, usecase.KindUsage, "patch.empty")
}

func TestPatchDeletedIsConflict(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Remove(ts.Session, "ME-1"); err != nil {
		t.Fatal(err)
	}
	body := "x"
	_, err := tasks.Patch(ts.Session, "ME-1", tasks.PatchInput{Body: &body})
	usecasetest.WantErr(t, err, usecase.KindConflict, "task.deleted")
}

func TestPatchUnknownKeyIsInvalid(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	_, err := tasks.Patch(ts.Session, "ME-1", tasks.PatchInput{Assigns: []usecase.Assign{{Key: "nope", Op: "=", Value: "x"}}})
	usecasetest.WantErr(t, err, usecase.KindInvalid, "assign.unknown_key")
}
