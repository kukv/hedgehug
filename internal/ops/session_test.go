package ops

import (
	"testing"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

func TestUpdateCommitsAndCallsAfterWrite(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	var after int
	ts.AfterWrite = func(*store.Store, config.StoreEntry) { after++ }
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
	ts := newTestSession(t)
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	_, err = ts.Resolve(ix, "ME-99")
	if KindOf(err) != KindNotFound {
		t.Fatalf("KindOf(%v) = %v; want KindNotFound", err, KindOf(err))
	}
	if got, want := err.Error(), `"ME-99": not found`; got != want {
		t.Errorf("Error() = %q; want %q", got, want)
	}
}
