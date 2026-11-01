package ops

import "testing"

// headOp is the Hedgehug-Op of the commit at HEAD of the test store.
func headOp(t *testing.T, ts *testSession) string {
	t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	log, err := s.Repo().Log(nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	op, _ := log[0].Trailer("Hedgehug-Op")
	return op
}

func TestPatchSetsFieldsAndBodyInOneCommit(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	before, err := s.Repo().Log(nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	body := "new body\n"
	res, err := ts.Patch("ME-1", PatchInput{
		Assigns: []Assign{{Key: "title", Op: "=", Value: "b"}, {Key: "status", Op: "=", Value: "in_progress"}},
		Body:    &body,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Committed || res.Commit == "" {
		t.Fatalf("result = %+v, want a commit", res)
	}
	after, err := s.Repo().Log(nil, 0)
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
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	body := "only the body\n"
	if _, err := ts.Patch("ME-1", PatchInput{Body: &body}); err != nil {
		t.Fatal(err)
	}
	if op := headOp(t, ts); op != "edit" {
		t.Errorf("op = %q, want edit", op)
	}
}

func TestPatchNothingIsUsage(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	_, err := ts.Patch("ME-1", PatchInput{})
	wantErr(t, err, KindUsage, "patch.empty")
}

func TestPatchDeletedIsConflict(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Remove("ME-1"); err != nil {
		t.Fatal(err)
	}
	body := "x"
	_, err := ts.Patch("ME-1", PatchInput{Body: &body})
	wantErr(t, err, KindConflict, "task.deleted")
}

func TestPatchUnknownKeyIsInvalid(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	_, err := ts.Patch("ME-1", PatchInput{Assigns: []Assign{{Key: "nope", Op: "=", Value: "x"}}})
	wantErr(t, err, KindInvalid, "assign.unknown_key")
}
