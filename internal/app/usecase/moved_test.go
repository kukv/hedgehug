package usecase_test

import (
	"errors"
	"maps"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/browse"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func wantMoved(t *testing.T, ts *usecasetest.Session, err error, args map[string]any) {
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
	var oe *usecase.Error
	if !errors.As(err, &oe) || oe.Kind != usecase.KindNotFound || oe.Code != "task.moved" {
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
	ts := usecasetest.NewTwoStore(t)
	ulid := usecasetest.MovedFixture(t, ts)
	_, err := browse.Show(ts.Session, "org-a:API-1")
	wantMoved(t, ts, err, map[string]any{"input": "API-1", "store": "org-b", "id": "WEB-2"})
	// the ULID too; Args carry the ULID of the store it went to
	_, err = browse.Show(ts.Session, "org-a:"+ulid.String())
	wantMoved(t, ts, err, nil)
	// without --store the task is found where it is now
	if res, err := browse.Show(ts.Session, "API-1"); err != nil || res.ID != "WEB-2" {
		t.Errorf("Show(API-1) = %+v, %v; want org-b WEB-2", res, err)
	}
	// a session of one store sees the forward too
	_, err = browse.Show(ts.ForStore("org-a"), "API-1")
	wantMoved(t, ts, err, nil)
}

func TestShowAMovedTaskOfAStoreNotMounted(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	usecasetest.MovedFixture(t, ts)
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
	ts.DropMount()
	_, err = browse.Show(ts.ForStore("org-a"), "API-1")
	var oe *usecase.Error
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
	ts := usecasetest.NewTwoStore(t)
	if _, err := tasks.Add(ts.ForStore("org-a"), tasks.AddInput{Title: "second"}); err != nil {
		t.Fatal(err)
	}
	usecasetest.MovedFixture(t, ts)
	_, err := tasks.Dep(ts.ForStore("org-a"), "API-2", "API-1", true)
	wantMoved(t, ts, err, map[string]any{"store": "org-b"})
}

func TestUndoRefusesMoves(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ op, why string }{
		{"move-in", "a move between stores cannot be undone; move it back with hedgehug mv --store"},
		{"move-out", "a move between stores cannot be undone; move it back with hedgehug mv --store"},
		{"purge", "a purge cannot be undone"},
	} {
		t.Run(tc.op, func(t *testing.T) {
			ts := usecasetest.NewTwoStore(t)
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
			before := usecasetest.HeadOf(t, s)
			_, err = tasks.Undo(alice)
			var oe *usecase.Error
			if !errors.As(err, &oe) || oe.Kind != usecase.KindConflict || oe.Code != "undo.blocked" || err.Error() != tc.why {
				t.Fatalf("Undo = %v; want KindConflict undo.blocked %q", err, tc.why)
			}
			if oe.Args["op"] != tc.op {
				t.Errorf("Args = %v; want op %q", oe.Args, tc.op)
			}
			if after := usecasetest.HeadOf(t, s); after != before {
				t.Errorf("HEAD moved from %s to %s", before, after)
			}
		})
	}
}

func TestUndoOfATaskThatMovedSince(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	alice := ts.ForStore("org-a")
	s, err := alice.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Set(alice, "API-1", []usecase.Assign{{Key: "title", Op: "=", Value: "renamed"}}); err != nil {
		t.Fatal(err)
	}
	usecasetest.MovedFixture(t, ts)
	before := usecasetest.HeadOf(t, s)
	_, err = tasks.Undo(alice)
	wantMoved(t, ts, err, map[string]any{"store": "org-b", "id": "WEB-2"})
	if after := usecasetest.HeadOf(t, s); after != before {
		t.Errorf("HEAD moved from %s to %s", before, after)
	}
}

// hh use names where a project went, as the other commands do.
func TestUseAMovedProject(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	usecasetest.MovedFixture(t, ts)
	_, err := stores.Use(ts.ForStore("org-a"), "API-1")
	wantMoved(t, ts, err, map[string]any{"input": "API-1"})
}

func hasKey(m map[string]any, k string) bool { _, ok := m[k]; return ok }

// A forward the index cannot name still gives task.moved, with the keys of
// the destination left out.
func TestMovedUnknownErr(t *testing.T) {
	t.Parallel()
	ulid := model.NewULID()
	err := usecase.MovedUnknownErr(ulid)
	var oe *usecase.Error
	if !errors.As(err, &oe) || oe.Kind != usecase.KindNotFound || oe.Code != "task.moved" {
		t.Fatalf("err = %v; want KindNotFound task.moved", err)
	}
	if oe.Args["input"] != ulid.String() || len(oe.Args) != 1 {
		t.Errorf("Args = %v; want only input", oe.Args)
	}
	if want := ulid.String() + " moved to another store"; err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
}
