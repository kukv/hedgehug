package ops

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
)

// order returns the ids of the live tasks under parent in the first workspace.
func (ts *testSession) order(parent *model.ULID) []string {
	ts.t.Helper()
	_, ix, err := ts.Open()
	if err != nil {
		ts.t.Fatal(err)
	}
	got := []string{}
	for _, e := range Siblings(ix, parent, ix.Workspaces()[0].ULID, model.ULID{}) {
		got = append(got, e.ID)
	}
	return got
}

func TestMoveParentAndOrder(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	for _, title := range []string{"a", "b", "c"} {
		if _, err := ts.Add(AddInput{Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	res, err := ts.Move(MoveInput{Arg: "ME-3", Before: "ME-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Committed || res.Moved || res.Index == nil {
		t.Errorf("result = committed %v moved %v index %v; want committed, not moved, an index", res.Committed, res.Moved, res.Index)
	}
	if len(res.Tasks) != 1 || res.Tasks[0].ID != "ME-3" || !slices.Equal(res.OldIDs, []string{"ME-3"}) {
		t.Errorf("tasks = %v old ids = %v; want just ME-3", res.Tasks, res.OldIDs)
	}
	if got, want := strings.Join(ts.order(nil), " "), "ME-3 ME-1 ME-2"; got != want {
		t.Errorf("order = %s; want %s", got, want)
	}
	if _, err := ts.Move(MoveInput{Arg: "ME-3", After: "ME-1"}); err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(ts.order(nil), " "), "ME-1 ME-3 ME-2"; got != want {
		t.Errorf("order after --after = %s; want %s", got, want)
	}
	if _, err := ts.Move(MoveInput{Arg: "ME-1", Parent: "ME-2"}); err != nil {
		t.Fatal(err)
	}
	_, ix, _ := ts.Open()
	p, _ := ts.Resolve(ix, "ME-2")
	if got, want := strings.Join(ts.order(&p.ULID), " "), "ME-1"; got != want {
		t.Errorf("children of ME-2 = %s; want %s", got, want)
	}
	// ME-1 is under ME-2 now, so ME-2 under ME-1 would be a cycle; the store
	// refuses it.
	_, err = ts.Move(MoveInput{Arg: "ME-2", Parent: "ME-1"})
	// The store's error is not wrapped in an *Error; classify gives it its Code.
	if c := classify(err); c == nil || c.Kind != KindInvalid || c.Code != "schema.violation" {
		t.Errorf("classify(%v) = %+v; want KindInvalid schema.violation", err, c)
	}
	if KindOf(err) != KindInvalid {
		t.Errorf("KindOf = %v; want KindInvalid", KindOf(err))
	}
	if err != nil && !strings.Contains(err.Error(), "leads back to this task") {
		t.Errorf("message = %q", err)
	}
	_, err = ts.Move(MoveInput{Arg: "ME-1", Root: true})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(ts.order(nil), " "), "ME-3 ME-2 ME-1"; got != want {
		t.Errorf("order after --root = %s; want %s", got, want)
	}
}

func TestMoveUnderParentOfMissingWorkspace(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	// A task whose workspace is not in the store, committed past the store's
	// checks as a sync or a hand edit could.
	_, e, err := ts.Entry()
	if err != nil {
		t.Fatal(err)
	}
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	ghost, gone := model.NewULID(), model.NewULID()
	rel := model.TaskPath(ghost)
	p := filepath.Join(e.Path, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	body := "---\nulid: " + ghost.String() + "\nid: ME-9\nworkspace: " + gone.String() + "\nkind: task\ntitle: ghost\nstatus: todo\n---\n"
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	author, err := config.CommitAuthor(config.Account{}, false, ts.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{rel}, Subject: "raw", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	_, err = ts.Move(MoveInput{Arg: "ME-1", Parent: "ME-9", Workspace: "me"})
	wantErr(t, err, KindInvalid, "move.invalid")
	if want := "the parent's workspace " + gone.String() + " is not in this store"; err != nil && err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
}

func TestMoveNoChangeIsNotCommitted(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	res, err := ts.Move(MoveInput{Arg: "ME-1", Root: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Committed {
		t.Error("moving the only task to the top level committed")
	}
}

func TestMoveToWorkspaceRenumbersSubtree(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	ts.addWorkspace("api", "API")
	for _, in := range []AddInput{{Title: "p", Workspace: "me"}, {Title: "c", Parent: "ME-1"}} {
		if _, err := ts.Add(in); err != nil {
			t.Fatal(err)
		}
	}
	res, err := ts.Move(MoveInput{Arg: "ME-1", Workspace: "api"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Moved {
		t.Fatal("Moved = false for a move to another workspace")
	}
	if !slices.Equal(res.OldIDs, []string{"ME-1", "ME-2"}) {
		t.Errorf("old ids = %v; want [ME-1 ME-2]", res.OldIDs)
	}
	var now []string
	for _, task := range res.Tasks {
		now = append(now, task.ID)
	}
	if !slices.Equal(now, []string{"API-1", "API-2"}) {
		t.Errorf("new ids = %v; want [API-1 API-2]", now)
	}
}

func TestMoveErrors(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	ts.addWorkspace("api", "API")
	for _, in := range []AddInput{{Title: "a", Workspace: "me"}, {Title: "b", Workspace: "me"}, {Title: "kid", Parent: "ME-2"}, {Title: "x", Workspace: "api"}} {
		if _, err := ts.Add(in); err != nil {
			t.Fatal(err)
		}
	}

	// a parent in another workspace than the one asked for
	_, err := ts.Move(MoveInput{Arg: "ME-1", Parent: "API-1", Workspace: "me"})
	wantErr(t, err, KindInvalid, "move.invalid")
	if err != nil && err.Error() != "the parent is in workspace api" {
		t.Errorf("message = %q", err)
	}

	// a sibling that is not under the same parent
	_, err = ts.Move(MoveInput{Arg: "ME-1", Before: "ME-3"})
	wantErr(t, err, KindInvalid, "move.invalid")
	if err != nil && (!strings.Contains(err.Error(), "ME-3") || !strings.Contains(err.Error(), "is not under the same parent")) {
		t.Errorf("message = %q", err)
	}

	_, err = ts.Move(MoveInput{Arg: "ME-9", Root: true})
	wantErr(t, err, KindNotFound, "task.not_found")

	ts.deleteTask("ME-1")
	_, err = ts.Move(MoveInput{Arg: "ME-1", Root: true})
	wantErr(t, err, KindConflict, "task.deleted")
	_, err = ts.Move(MoveInput{Arg: "ME-2", Parent: "ME-1"})
	wantErr(t, err, KindConflict, "task.parent_deleted")

	_, err = ts.Move(MoveInput{Arg: "ME-2", Workspace: "nope"})
	if err == nil {
		t.Fatal("moving to an unknown workspace succeeded")
	}
}
