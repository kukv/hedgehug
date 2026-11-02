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

// order returns the ids of the live tasks under parent in the workspace ws.
func (ts *testSession) order(ws model.ULID, parent *model.ULID) []string {
	ts.t.Helper()
	_, ix, err := ts.Open()
	if err != nil {
		ts.t.Fatal(err)
	}
	got := []string{}
	for _, e := range Siblings(ix, parent, ws, model.ULID{}) {
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
	if got, want := strings.Join(ts.order(ts.workspaceULID("me"), nil), " "), "ME-3 ME-1 ME-2"; got != want {
		t.Errorf("order = %s; want %s", got, want)
	}
	if _, err := ts.Move(MoveInput{Arg: "ME-3", After: "ME-1"}); err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(ts.order(ts.workspaceULID("me"), nil), " "), "ME-1 ME-3 ME-2"; got != want {
		t.Errorf("order after --after = %s; want %s", got, want)
	}
	if _, err := ts.Move(MoveInput{Arg: "ME-1", Parent: "ME-2"}); err != nil {
		t.Fatal(err)
	}
	_, ix, _ := ts.Open()
	p, _ := ts.Resolve(ix, "ME-2")
	if got, want := strings.Join(ts.order(ts.workspaceULID("me"), &p.ULID), " "), "ME-1"; got != want {
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
	if got, want := strings.Join(ts.order(ts.workspaceULID("me"), nil), " "), "ME-3 ME-2 ME-1"; got != want {
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

func TestAddBeforeAfter(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	me := ts.workspaceULID("me")
	for _, title := range []string{"a", "b", "c"} {
		if _, err := ts.Add(AddInput{Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	logLen := func() int {
		s, err := ts.OpenStore()
		if err != nil {
			t.Fatal(err)
		}
		l, err := s.Repo().Log(nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		return len(l)
	}
	n := logLen()
	res, err := ts.Add(AddInput{Title: "after a", After: "ME-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Committed || logLen() != n+1 {
		t.Errorf("committed %v, commits %d -> %d; want one new commit", res.Committed, n, logLen())
	}
	if got, want := strings.Join(ts.order(me, nil), " "), "ME-1 ME-4 ME-2 ME-3"; got != want {
		t.Errorf("order after --after = %s; want %s", got, want)
	}
	if _, err := ts.Add(AddInput{Title: "before c", Before: "ME-3"}); err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(ts.order(me, nil), " "), "ME-1 ME-4 ME-2 ME-5 ME-3"; got != want {
		t.Errorf("order after --before = %s; want %s", got, want)
	}
}

func TestAddAfterUnderParent(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	me := ts.workspaceULID("me")
	for _, in := range []AddInput{{Title: "p"}, {Title: "k1", Parent: "ME-1"}, {Title: "k2", Parent: "ME-1"}} {
		if _, err := ts.Add(in); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ts.Add(AddInput{Title: "mid", Parent: "ME-1", After: "ME-2"}); err != nil {
		t.Fatal(err)
	}
	p := ulidOf(t, ts, "ME-1")
	if got, want := strings.Join(ts.order(me, &p), " "), "ME-2 ME-4 ME-3"; got != want {
		t.Errorf("order = %s; want %s", got, want)
	}
}

func TestAddPositionErrors(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	ts.addWorkspace("api", "API")
	for _, in := range []AddInput{{Title: "a", Workspace: "me"}, {Title: "b", Workspace: "me"}, {Title: "kid", Parent: "ME-2"}, {Title: "x", Workspace: "api"}} {
		if _, err := ts.Add(in); err != nil {
			t.Fatal(err)
		}
	}
	ts.deleteTask("ME-1")
	for _, tc := range []struct {
		name string
		in   AddInput
		kind Kind
		code string
	}{
		{"under another parent", AddInput{Title: "t", Parent: "ME-2", Before: "ME-2"}, KindInvalid, "add.position"},
		{"deleted", AddInput{Title: "t", Workspace: "me", After: "ME-1"}, KindInvalid, "add.position"},
		{"other workspace", AddInput{Title: "t", Workspace: "me", After: "API-1"}, KindInvalid, "add.position"},
		{"both", AddInput{Title: "t", Workspace: "me", Before: "ME-2", After: "ME-2"}, KindUsage, "position.both"},
	} {
		before := ts.head()
		_, err := ts.Add(tc.in)
		wantErr(t, err, tc.kind, tc.code)
		if ts.head() != before {
			t.Errorf("%s: a failed add left a commit", tc.name)
		}
	}
	_, err := ts.Move(MoveInput{Arg: "ME-2", Before: "ME-2", After: "ME-2"})
	wantErr(t, err, KindUsage, "position.both")
}

// Siblings that share a rank are ordered by id and no rank lies between them,
// so the new task goes past the group on the side asked for.
func TestAddNextToSiblingsWithTheSameRank(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	me := ts.workspaceULID("me")
	for _, title := range []string{"a", "b", "d"} {
		if _, err := ts.Add(AddInput{Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	a, b := ts.addedTask("ME-1"), ts.addedTask("ME-2")
	ts.editRawFile(model.TaskPath(b.ULID), "rank: "+string(b.Rank), "rank: "+string(a.Rank))
	if _, err := ts.Add(AddInput{Title: "after A", After: "ME-1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Add(AddInput{Title: "before B", Before: "ME-2"}); err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(ts.order(me, nil), " "), "ME-5 ME-1 ME-2 ME-4 ME-3"; got != want {
		t.Errorf("order = %s; want %s", got, want)
	}
}

// addedTask is the task id names, as the store has it.
func (ts *testSession) addedTask(id string) *model.Task {
	ts.t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		ts.t.Fatal(err)
	}
	r, err := ts.Show(id)
	if err != nil {
		ts.t.Fatal(err)
	}
	tk, err := s.Task(r.Entry.ULID)
	if err != nil {
		ts.t.Fatal(err)
	}
	return tk
}

// A move to where the task already is keeps its rank, so nothing is committed;
// a move that changes the order still commits.
func TestMoveToTheSamePlaceIsNotCommitted(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	me := ts.workspaceULID("me")
	for _, title := range []string{"a", "b", "c"} {
		if _, err := ts.Add(AddInput{Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		name string
		in   MoveInput
		want bool // committed
		list string
	}{
		{"after the task just before", MoveInput{Arg: "ME-2", After: "ME-1"}, false, "ME-1 ME-2 ME-3"},
		{"before the task just after", MoveInput{Arg: "ME-2", Before: "ME-3"}, false, "ME-1 ME-2 ME-3"},
		{"to the end when last", MoveInput{Arg: "ME-3", Root: true}, false, "ME-1 ME-2 ME-3"},
		{"before a task two places on", MoveInput{Arg: "ME-1", Before: "ME-3"}, true, "ME-2 ME-1 ME-3"},
		{"after a task two places on", MoveInput{Arg: "ME-2", After: "ME-3"}, true, "ME-1 ME-3 ME-2"},
		{"to the end when not last", MoveInput{Arg: "ME-1", Root: true}, true, "ME-3 ME-2 ME-1"},
	} {
		res, err := ts.Move(c.in)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if res.Committed != c.want {
			t.Errorf("%s: committed = %v; want %v", c.name, res.Committed, c.want)
		}
		if got := strings.Join(ts.order(me, nil), " "); got != c.list {
			t.Errorf("%s: order = %s; want %s", c.name, got, c.list)
		}
	}
}

// Tasks of one rank are ordered by id and a move next to one of them goes past
// the group, so it is not a move to where the task is.
func TestMoveNextToATaskOfTheSameRankStillMoves(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	me := ts.workspaceULID("me")
	for _, title := range []string{"a", "b", "c"} {
		if _, err := ts.Add(AddInput{Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	a, b := ts.addedTask("ME-1"), ts.addedTask("ME-2")
	ts.editRawFile(model.TaskPath(b.ULID), "rank: "+string(b.Rank), "rank: "+string(a.Rank))
	res, err := ts.Move(MoveInput{Arg: "ME-2", After: "ME-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Committed {
		t.Error("moving past a task of the same rank did not commit")
	}
	if got, want := strings.Join(ts.order(me, nil), " "), "ME-1 ME-2 ME-3"; got != want {
		t.Errorf("order = %s; want %s", got, want)
	}
}
