package statedb

import (
	"context"
	"maps"
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
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
	own := port.SaveKey{Version: "v3", Author: "alice", At: k.At, Op: "op3"}
	if err := d.JudgedSaves().MarkOwn(ctx, "S1", []port.SaveKey{own}); err != nil {
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
	if ok, err := d.JudgedSaves().Own(ctx, "S1", "op3"); err != nil || !ok {
		t.Errorf("Own after reopen = %v, %v", ok, err)
	}
}

// Received は、op の記録があり、同じ op にこのマシンの保存の記録がないとき true(決定 C4d-16)。別の Store の記録は数えない。
func TestReceivedOps(t *testing.T) {
	d := openTemp(t)
	ctx := t.Context()
	at := time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)
	key := func(v string, op change.ID) port.SaveKey {
		return port.SaveKey{Version: v, Op: op, Author: "alice", At: at}
	}
	j := d.JudgedSaves()
	if err := j.Mark(ctx, "S1", []port.SaveKey{key("v1", "recv"), key("v3", "both")}); err != nil {
		t.Fatal(err)
	}
	if err := j.MarkOwn(ctx, "S1", []port.SaveKey{key("v2", "own"), key("v4", "both")}); err != nil {
		t.Fatal(err)
	}
	if err := j.Mark(ctx, "S2", []port.SaveKey{key("v5", "other")}); err != nil {
		t.Fatal(err)
	}
	got, err := d.Received(ctx, "S1", []change.ID{"recv", "own", "both", "none", "other", ""})
	if err != nil {
		t.Fatal(err)
	}
	want := map[change.ID]bool{"recv": true}
	if !maps.Equal(got, want) {
		t.Errorf("Received = %v, want %v", got, want)
	}
	if got, err := d.Received(ctx, "S1", nil); err != nil || len(got) != 0 {
		t.Errorf("Received(nil) = %v, %v", got, err)
	}
}
