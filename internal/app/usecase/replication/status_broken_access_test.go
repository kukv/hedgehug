package replication_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// A broken access.json is a reason in ReadOnly, and the rest of the row (the
// unpushed count, the times) is still filled.
func TestSyncStatusBrokenAccessKeepsTheRow(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	usecasetest.AddBareRemote(t, ts)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "x"}); err != nil {
		t.Fatal(err)
	}
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.WriteLocal("access.json", []byte("{not json")); err != nil {
		t.Fatal(err)
	}
	st, err := usecase.SyncStatus(ts.Session)
	if err != nil || len(st) != 1 {
		t.Fatalf("SyncStatus = %+v, %v", st, err)
	}
	if st[0].Error != "" {
		t.Errorf("Error = %q; want none", st[0].Error)
	}
	if st[0].ReadOnly != "access.json is broken; run hedgehug sync" {
		t.Errorf("ReadOnly = %q", st[0].ReadOnly)
	}
	if st[0].Unpushed == nil || *st[0].Unpushed == 0 {
		t.Errorf("Unpushed = %v; want it filled", st[0].Unpushed)
	}
}
