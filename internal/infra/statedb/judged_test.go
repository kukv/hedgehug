package statedb

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

func TestJudgedSavesContract(t *testing.T) {
	d := openTemp(t)
	apptest.RunJudgedSavesContract(t, func() port.JudgedSaves { return d.JudgedSaves() })
}

func TestJudgedSavesSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	k := port.SaveKey{Version: "v1", Author: "alice", At: time.Date(2026, 11, 14, 9, 0, 0, 5, time.UTC), Op: "op1"}
	if err := d.JudgedSaves().Mark(ctx, "S1", []port.SaveKey{k}); err != nil {
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
	if ok, err := d.JudgedSaves().Started(ctx, "S1"); err != nil || !ok {
		t.Errorf("Started after reopen = %v, %v", ok, err)
	}
	k.Version = "v2"
	if m, err := d.JudgedSaves().Known(ctx, "S1", k); err != nil || m != port.SameSave {
		t.Errorf("Known after reopen = %v, %v", m, err)
	}
}
