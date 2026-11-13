package apptest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
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

func TestReplicatorSyncSave(t *testing.T) {
	clock := apptest.NewClock(time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC))
	stores := apptest.NewStores(clock, apptest.NewIDs())
	for _, id := range []port.StoreID{"S1", "S2", "S3"} {
		stores.AddStore(id)
	}
	boom := errors.New("boom")
	r := &apptest.Replicator{
		Stores:   stores,
		Results:  map[port.StoreID]port.SyncResult{"S1": {Sent: 1, Seen: "v9"}},
		Errs:     map[port.StoreID]error{"S3": port.ErrAuthFailed},
		SendErrs: map[port.StoreID]error{"S2": port.ErrWriteDenied},
	}
	op := port.Operation{Actor: change.User("taro"), Summary: "追加"}
	join := func(tx port.Tx) error { return tx.WriteMembers().Append(member.Init(), member.Join("taro")) }
	logins := func(id port.StoreID) int {
		n := 0
		_ = stores.Read(t.Context(), id, func(r port.Reader) error { ms, _ := r.Members().List(); n = len(ms); return nil })
		return n
	}
	if res, err := r.SyncSave(t.Context(), "S1", port.Remote{URL: "u", Token: "tok"}, op, join); err != nil || res.Seen != "v9" || logins("S1") != 1 {
		t.Errorf("S1 = %+v, %v, members %d", res, err, logins("S1"))
	}
	var se *port.SaveError
	if _, err := r.SyncSave(t.Context(), "S2", port.Remote{}, op, join); !errors.Is(err, port.ErrWriteDenied) || errors.As(err, &se) || logins("S2") != 0 {
		t.Errorf("S2 = %v, members %d, want ErrWriteDenied (not a SaveError) and nothing saved", err, logins("S2"))
	}
	called := false
	if _, err := r.SyncSave(t.Context(), "S3", port.Remote{}, op, func(port.Tx) error { called = true; return nil }); !errors.Is(err, port.ErrAuthFailed) || called {
		t.Errorf("S3 = %v, fn called %v, want ErrAuthFailed and no call", err, called)
	}
	_, err := r.SyncSave(t.Context(), "S1", port.Remote{}, op, func(port.Tx) error { return boom })
	if !errors.As(err, &se) || !errors.Is(err, boom) {
		t.Errorf("fn error = %v, want a SaveError around boom", err)
	}
	if calls := r.Calls(); len(calls) != 4 || !calls[0].Save || calls[0].Remote.Token != "tok" || calls[0].Mirror {
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
