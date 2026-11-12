package statedb

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

func openTemp(t *testing.T) *DB {
	t.Helper()
	d, err := Open(filepath.Join(t.TempDir(), "hedgehug", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestSyncStateContract(t *testing.T) {
	apptest.RunSyncStateContract(t, func(t *testing.T) port.SyncState { return openTemp(t).SyncState() })
}

func TestSyncStateSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	at := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := d.SyncState().Succeeded(ctx, "S1", at, "github.com/o/r", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := d.SyncState().Failed(ctx, "S1", at.Add(time.Minute), syncstatus.Rewritten); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	d, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	rs, err := d.SyncState().Records(ctx)
	if err != nil || len(rs) != 1 || rs[0].Reason != syncstatus.Rewritten || !rs[0].LastSuccess.Equal(at) {
		t.Errorf("Records after reopen = %+v, %v", rs, err)
	}
	if seen, _ := d.SyncState().Seen(ctx, "S1", "github.com/o/r"); seen != "v1" {
		t.Errorf("Seen after reopen = %q", seen)
	}
}
