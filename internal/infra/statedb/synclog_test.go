package statedb

import (
	"context"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
)

// 決定 C4d-13: 送信・受信が両方 0 の同期は書かない。List は Store ごとに新しい順(同じ時刻なら後に書いたものが先)。
func TestSyncLogAddList(t *testing.T) {
	d := openTemp(t)
	l := d.SyncLog()
	ctx := context.Background()
	base := time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)
	adds := []struct {
		store          port.StoreID
		at             time.Time
		sent, received int
	}{
		{"S1", base, 2, 1},
		{"S1", base.Add(time.Second), 0, 0},
		{"S1", base.Add(500 * time.Millisecond), 0, 3}, // 末尾の 0 を削る書き方だと、文字の順で base より前になる
		{"S2", base.Add(time.Hour), 1, 0},
		{"S1", base, 4, 0},
	}
	for _, a := range adds {
		if err := l.Add(ctx, a.store, a.at, a.sent, a.received); err != nil {
			t.Fatal(err)
		}
	}
	got, err := l.List(ctx, "S1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("List = %+v, want 3", got)
	}
	want := []struct {
		at             time.Time
		sent, received int
	}{{base.Add(500 * time.Millisecond), 0, 3}, {base, 4, 0}, {base, 2, 1}}
	for i, w := range want {
		g := got[i]
		if g.Store != "S1" || !g.At.Equal(w.at) || g.Sent != w.sent || g.Received != w.received || g.ID == 0 {
			t.Errorf("List[%d] = %+v, want at %v sent %d received %d", i, g, w.at, w.sent, w.received)
		}
	}
	if got[1].ID <= got[2].ID {
		t.Errorf("IDs = %d, %d; want the later line to have the larger ID", got[1].ID, got[2].ID)
	}
	if s2, err := l.List(ctx, "S2"); err != nil || len(s2) != 1 || s2[0].Sent != 1 {
		t.Errorf("List(S2) = %+v, %v", s2, err)
	}
	if none, err := l.List(ctx, "S9"); err != nil || len(none) != 0 {
		t.Errorf("List(S9) = %+v, %v", none, err)
	}
}

// ForgetStore は、その Store の行だけを消す。消したあとに足した行は、前の番号を使い回さない。
func TestSyncLogForgetStore(t *testing.T) {
	d := openTemp(t)
	l := d.SyncLog()
	ctx := context.Background()
	at := time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)
	for _, s := range []port.StoreID{"S1", "S2"} {
		if err := l.Add(ctx, s, at, 1, 1); err != nil {
			t.Fatal(err)
		}
	}
	before, _ := l.List(ctx, "S2")
	if err := l.ForgetStore(ctx, "S2"); err != nil {
		t.Fatal(err)
	}
	if got, err := l.List(ctx, "S2"); err != nil || len(got) != 0 {
		t.Errorf("List(S2) after ForgetStore = %+v, %v", got, err)
	}
	if got, err := l.List(ctx, "S1"); err != nil || len(got) != 1 {
		t.Errorf("List(S1) after ForgetStore(S2) = %+v, %v", got, err)
	}
	if err := l.Add(ctx, "S2", at, 1, 0); err != nil {
		t.Fatal(err)
	}
	if got, _ := l.List(ctx, "S2"); len(got) != 1 || len(before) != 1 || got[0].ID <= before[0].ID {
		t.Errorf("ID after ForgetStore = %+v, before %+v; want a new ID", got, before)
	}
}
