package apptest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/repair"
)

func TestReplicatorRecordsCallsAndReturnsWhatWasSet(t *testing.T) {
	boom := errors.New("boom")
	r := &apptest.Replicator{
		Results:    map[port.StoreID]port.SyncResult{"S1": {Sent: 1, Received: 2, Seen: "v1"}},
		Errs:       map[port.StoreID]error{"S2": port.ErrAuthFailed},
		MirrorErrs: map[string]error{"https://m.invalid/x": boom},
		UnsentN:    map[port.StoreID]int{"S1": 4},
	}
	ctx := context.Background()
	if res, err := r.Sync(ctx, "S1", port.Remote{URL: "u"}); err != nil || res.Sent != 1 || res.Seen != "v1" {
		t.Errorf("Sync S1 = %+v, %v", res, err)
	}
	if _, err := r.Sync(ctx, "S2", port.Remote{}); !errors.Is(err, port.ErrAuthFailed) {
		t.Errorf("Sync S2 = %v", err)
	}
	if err := r.Mirror(ctx, "S1", port.Remote{URL: "https://m.invalid/x"}); !errors.Is(err, boom) {
		t.Errorf("Mirror = %v", err)
	}
	if n, err := r.Unsent(ctx, "S1"); n != 4 || err != nil {
		t.Errorf("Unsent = %d, %v", n, err)
	}
	calls := r.Calls()
	if len(calls) != 3 || calls[0].Store != "S1" || calls[0].Remote.URL != "u" || !calls[2].Mirror {
		t.Errorf("Calls = %+v", calls)
	}
}

func TestRepairWriterRecordsOpsThatLeftRows(t *testing.T) {
	ctx := context.Background()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore("S1")
	if err := apptest.Sample(ctx, s, "S1"); err != nil {
		t.Fatal(err)
	}
	w := &apptest.RepairWriter{Stores: s, Store: "S1", F: repair.Facts{Ours: map[change.ID]bool{"o": true}}}
	var _ port.RepairWriter = w
	if w.Facts().Side("o") != 1 {
		t.Errorf("facts = %+v", w.Facts())
	}
	op := func(summary string) port.Operation {
		return port.Operation{Actor: change.Actor{Kind: change.ActorSync}, Summary: summary}
	}
	if err := w.Write(op("行なし"), func(tx port.Tx) error { return tx.WriteItems().Append(apptest.SampleTask) }); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	if err := w.Write(op("失敗"), func(tx port.Tx) error {
		_ = tx.WriteItems().Append(apptest.SampleTask, change.NewSet("title", "ログイン画面を作る", "X"))
		return boom
	}); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if err := w.Write(op("修復"), func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("title", "ログイン画面を作る", "R"))
	}); err != nil {
		t.Fatal(err)
	}
	if len(w.Ops) != 1 || w.Ops[0].Summary != "修復" {
		t.Fatalf("ops = %+v", w.Ops)
	}
	var title string
	if err := w.Read(func(r port.Reader) error {
		it, err := r.Items().Get(apptest.SampleTask)
		title = it.Title
		return err
	}); err != nil || title != "R" {
		t.Fatalf("read title = %q, %v", title, err)
	}
}
