package usecase_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestUpdateCommitsAndCallsAfterWrite(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	var after int
	ts.AfterWrite = func(string) { after++ }
	s, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	ws := ix.Workspaces()[0]
	task := &model.Task{ULID: model.NewULID(), Title: "a", Kind: "task", Status: "todo", Workspace: ws.ULID}
	committed, err := ts.Update(s, store.Op{Name: "add", Task: task.ULID}, func(tx *store.Tx) error { return tx.PutTask(task) })
	if err != nil || !committed {
		t.Fatalf("Update = %v, %v; want true, nil", committed, err)
	}
	if after != 1 {
		t.Errorf("AfterWrite called %d times; want 1", after)
	}
}

func TestResolveNotFoundIsKindNotFound(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	_, err = ts.Resolve(ix, "ME-99")
	if usecase.KindOf(err) != usecase.KindNotFound {
		t.Fatalf("KindOf(%v) = %v; want KindNotFound", err, usecase.KindOf(err))
	}
	if got, want := err.Error(), `"ME-99": not found`; got != want {
		t.Errorf("Error() = %q; want %q", got, want)
	}
}

// A write to a store with a remote starts the background sync once when the
// session asks for it (BackgroundSync), and never when it does not. It does
// not run in parallel: replicate.SetSpawn replaces what Start does for all.
func TestBackgroundSyncStartsOnlyWhenAsked(t *testing.T) {
	for _, tc := range []struct {
		background bool
		want       int
	}{{false, 0}, {true, 1}} {
		ts := usecasetest.New(t)
		usecasetest.AddBareRemote(t, ts)
		ts.Env["HEDGEHUG_AUTO_SYNC"] = "1"
		ts.BackgroundSync = tc.background
		var started int
		defer replicate.SetSpawn(func(string, []string, string) error { started++; return nil })()
		s, ix, err := ts.Open()
		if err != nil {
			t.Fatal(err)
		}
		task := &model.Task{ULID: model.NewULID(), Title: "a", Kind: "task", Status: "todo", Workspace: ix.Workspaces()[0].ULID}
		if _, err := ts.Update(s, store.Op{Name: "add", Task: task.ULID}, func(tx *store.Tx) error { return tx.PutTask(task) }); err != nil {
			t.Fatal(err)
		}
		if started != tc.want {
			t.Errorf("BackgroundSync=%v: the background sync started %d times; want %d", tc.background, started, tc.want)
		}
	}
}
