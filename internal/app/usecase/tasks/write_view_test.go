package tasks_test

import (
	"encoding/json"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// An undo that found nothing answers with the empty write, and its JSON has
// every key.
func TestUndoWriteOfNothing(t *testing.T) {
	t.Parallel()
	w := tasks.UndoResult{}.Write()
	if w.Undone != nil || w.Subject != "" || w.Changed || w.Commit != nil || w.Kept == nil || len(w.Kept) != 0 {
		t.Errorf("UndoResult{}.Write() = %+v", w)
	}
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"undone":null,"subject":"","changed":false,"commit":null,"kept":[]}`; string(b) != want {
		t.Errorf("json = %s; want %s", b, want)
	}
}

// An undo answers with the commit it took back, as a LogView.
func TestUndoWriteHasTheCommitTakenBack(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	r, err := tasks.Set(ts.Session, "ME-1", []usecase.Assign{{Key: "title", Op: "=", Value: "b"}})
	if err != nil {
		t.Fatal(err)
	}
	res, err := tasks.Undo(ts.Session)
	if err != nil {
		t.Fatal(err)
	}
	w := res.Write()
	if w.Undone == nil || w.Undone.Hash != r.Commit || w.Subject != "set ME-1 b" || !w.Changed || w.Commit == nil || *w.Commit != res.NewCommit {
		t.Errorf("Write() = %+v (undone %+v); want the set taken back", w, w.Undone)
	}
}

// rm and restore answer with the task named first and each task once, with
// the progress a list row has.
func TestAffectedWriteStartsWithTheTaskAndHasNoDuplicates(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	for _, in := range []tasks.AddInput{{Title: "p", Kind: "project"}, {Title: "c", Parent: "ME-1"}, {Title: "g", Parent: "ME-2"}} {
		if _, err := tasks.Add(ts.Session, in); err != nil {
			t.Fatal(err)
		}
	}
	rm, err := tasks.Remove(ts.Session, "ME-1")
	if err != nil {
		t.Fatal(err)
	}
	w, err := rm.Write()
	if err != nil || len(w.Tasks) != 3 || w.Tasks[0].ID != "ME-1" || w.Commit == nil || *w.Commit != rm.Commit {
		t.Fatalf("rm Write() = %+v, %v", w, err)
	}
	seen := map[string]bool{}
	for _, v := range w.Tasks {
		if seen[v.ULID] || v.DeletedAt == nil {
			t.Errorf("rm: %s twice, or not deleted: %+v", v.ID, v)
		}
		seen[v.ULID] = true
	}
	if w.Tasks[0].Progress == nil {
		t.Errorf("rm: the project has no progress")
	}
	rs, err := tasks.Restore(ts.Session, "ME-1")
	if err != nil {
		t.Fatal(err)
	}
	if w, err := rs.Write(); err != nil || len(w.Tasks) != 3 || w.Tasks[0].ID != "ME-1" || w.Tasks[0].DeletedAt != nil {
		t.Errorf("restore Write() = %+v, %v", w, err)
	}
}

// A move to another workspace answers with the task as it is now, and says
// the ids renumbered.
func TestMoveWriteNamesTheRenumbering(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddWorkspace("api", "API")
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a", Workspace: "me"}); err != nil {
		t.Fatal(err)
	}
	res, err := tasks.Move(ts.Session, tasks.MoveInput{Arg: "ME-1", Workspace: "api"})
	if err != nil {
		t.Fatal(err)
	}
	w, err := res.Write()
	if err != nil || w.Task.ID != "API-1" || w.Commit == nil {
		t.Fatalf("Write() = %+v, %v", w, err)
	}
	if got := res.Renumbered(); len(got) != 1 || got[0] != (usecase.IDChange{Old: "ME-1", New: "API-1"}) {
		t.Errorf("Renumbered() = %v; want ME-1 → API-1", got)
	}
}
