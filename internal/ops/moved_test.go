package ops

import (
	"errors"
	"maps"
	"testing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

var movedAuthor = gitrepo.Signature{Name: "Alice", Email: "alice@example.com"}

// movedFixture moves org-a's API-1 to org-b: org-b gets the task as WEB-2
// (API-1 as a former id), and org-a's file becomes a forward.
// It returns the ULID of the task.
func movedFixture(t *testing.T, ts *testSession) model.ULID {
	t.Helper()
	sa, err := ts.ForStore("org-a").OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	sb, ix, err := ts.ForStore("org-b").Open()
	if err != nil {
		t.Fatal(err)
	}
	ea, err := ts.ForStore("org-a").Resolve(mustIndex(t, ts, "org-a"), "API-1")
	if err != nil {
		t.Fatal(err)
	}
	ulid := ea.ULID
	nt := &model.Task{ULID: ulid, Title: "moved", Kind: "task", Status: "todo", Workspace: ix.Workspaces()[0].ULID, FormerIDs: []string{"API-1"}}
	if _, err := ts.ForStore("org-b").UpdateCommit(sb, store.Op{Name: "move-in", Task: ulid}, func(tx *store.Tx) error { return tx.PutTask(nt) }); err != nil {
		t.Fatal(err)
	}
	fwd := "---\nulid: " + ulid.String() + "\nmoved_to: " + sb.Meta().ULID.String() + "\n---\n"
	if _, err := sa.Update(store.Op{Name: "move-out", Author: movedAuthor, Detail: "API-1 org-a task"}, func(tx *store.Tx) error {
		return tx.PutFile(model.TaskPath(ulid), []byte(fwd))
	}); err != nil {
		t.Fatal(err)
	}
	return ulid
}

func mustIndex(t *testing.T, ts *testSession, name string) *index.Index {
	t.Helper()
	_, ix, err := ts.ForStore(name).Open()
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

func wantMoved(t *testing.T, ts *testSession, err error, args map[string]any) {
	t.Helper()
	bs, serr := ts.ForStore("org-b").OpenStore()
	if serr != nil {
		t.Fatal(serr)
	}
	args = maps.Clone(args)
	if args == nil {
		args = map[string]any{}
	}
	args["store"], args["id"], args["store_ulid"] = "org-b", "WEB-2", bs.Meta().ULID.String()
	var oe *Error
	if !errors.As(err, &oe) || oe.Kind != KindNotFound || oe.Code != "task.moved" {
		t.Fatalf("err = %v; want KindNotFound task.moved", err)
	}
	if text := "API-1 moved to org-b (org-b:WEB-2)"; err.Error() != text {
		t.Errorf("message = %q; want %q", err, text)
	}
	for k, v := range args {
		if oe.Args[k] != v {
			t.Errorf("Args[%s] = %v; want %v", k, oe.Args[k], v)
		}
	}
}

func TestShowAMovedTaskSaysWhereItWent(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	ulid := movedFixture(t, ts)
	_, err := ts.Show("org-a:API-1")
	wantMoved(t, ts, err, map[string]any{"input": "API-1", "store": "org-b", "id": "WEB-2"})
	// the ULID too; Args carry the ULID of the store it went to
	_, err = ts.Show("org-a:" + ulid.String())
	wantMoved(t, ts, err, nil)
	// without --store the task is found where it is now
	if res, err := ts.Show("API-1"); err != nil || res.Entry.ID != "WEB-2" {
		t.Errorf("Show(API-1) = %+v, %v; want org-b WEB-2", res.Entry, err)
	}
	// a session of one store sees the forward too
	_, err = ts.ForStore("org-a").Show("API-1")
	wantMoved(t, ts, err, nil)
}

func TestShowAMovedTaskOfAStoreNotMounted(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	movedFixture(t, ts)
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveStore("org-b"); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	ts.dropMount()
	_, err = ts.ForStore("org-a").Show("API-1")
	var oe *Error
	if !errors.As(err, &oe) || oe.Code != "task.moved" || hasKey(oe.Args, "store") || hasKey(oe.Args, "id") || oe.Args["store_ulid"] == nil {
		t.Fatalf("Show = %v (%+v); want task.moved with no store", err, oe)
	}
	want := "API-1 moved to another store (ULID " + oe.Args["store_ulid"].(string) + "); mount that store to follow it"
	if err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
}

func TestDepOnAMovedTask(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	if _, err := ts.ForStore("org-a").Add(AddInput{Title: "second"}); err != nil {
		t.Fatal(err)
	}
	movedFixture(t, ts)
	_, err := ts.ForStore("org-a").Dep("API-2", "API-1", true)
	wantMoved(t, ts, err, map[string]any{"store": "org-b"})
}

func TestUndoRefusesMovesAndRewrites(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ op, why string }{
		{"move-in", "a move between stores cannot be undone; move it back with hedgehug mv --store"},
		{"move-out", "a move between stores cannot be undone; move it back with hedgehug mv --store"},
		{"rewrite-history", "a history rewrite cannot be undone"},
	} {
		t.Run(tc.op, func(t *testing.T) {
			ts := newTwoStoreSession(t)
			alice := ts.ForStore("org-a")
			s, err := alice.OpenStore()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := alice.UpdateCommit(s, store.Op{Name: tc.op, Detail: "x"}, func(tx *store.Tx) error {
				tx.AllowEmpty()
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			before := headOf(t, s)
			_, err = alice.Undo()
			var oe *Error
			if !errors.As(err, &oe) || oe.Kind != KindConflict || oe.Code != "undo.blocked" || err.Error() != tc.why {
				t.Fatalf("Undo = %v; want KindConflict undo.blocked %q", err, tc.why)
			}
			if after := headOf(t, s); after != before {
				t.Errorf("HEAD moved from %s to %s", before, after)
			}
		})
	}
}

func TestUndoOfATaskThatMovedSince(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	alice := ts.ForStore("org-a")
	s, err := alice.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := alice.Set("API-1", []Assign{{Key: "title", Op: "=", Value: "renamed"}}); err != nil {
		t.Fatal(err)
	}
	movedFixture(t, ts)
	before := headOf(t, s)
	_, err = alice.Undo()
	wantMoved(t, ts, err, map[string]any{"store": "org-b", "id": "WEB-2"})
	if after := headOf(t, s); after != before {
		t.Errorf("HEAD moved from %s to %s", before, after)
	}
}

func headOf(t *testing.T, s *store.Store) string {
	t.Helper()
	h, err := s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	return h.String()
}

// hh use names where a project went, as the other commands do.
func TestUseAMovedProject(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	movedFixture(t, ts)
	_, err := ts.ForStore("org-a").Use("API-1")
	wantMoved(t, ts, err, map[string]any{"input": "API-1"})
}

func hasKey(m map[string]any, k string) bool { _, ok := m[k]; return ok }

// A forward the index cannot name still gives task.moved, with the keys of
// the destination left out.
func TestMovedUnknownErr(t *testing.T) {
	t.Parallel()
	ulid := model.NewULID()
	err := movedUnknownErr(ulid)
	var oe *Error
	if !errors.As(err, &oe) || oe.Kind != KindNotFound || oe.Code != "task.moved" {
		t.Fatalf("err = %v; want KindNotFound task.moved", err)
	}
	if oe.Args["input"] != ulid.String() || len(oe.Args) != 1 {
		t.Errorf("Args = %v; want only input", oe.Args)
	}
	if want := ulid.String() + " moved to another store"; err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
}
