package apptest

import (
	"errors"
	"maps"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

func fakeNotice(store port.StoreID, kind port.NoticeKind) port.Notice {
	return port.Notice{Kind: kind, Store: store, Workspace: "w1", Item: "i1", Text: "本文"}
}

func fakeList(t *testing.T, b port.InboxBook, stores ...port.StoreID) []port.NoticeRow {
	t.Helper()
	rows, err := b.List(t.Context(), port.NoticeQuery{Stores: stores, Limit: 300})
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestFakeInboxListPagesBySameTime(t *testing.T) {
	clock := NewClock(time.Unix(0, 0))
	clock.Step = 0
	b := &Inbox{Clock: clock}
	for range 5 {
		_ = b.Add(t.Context(), fakeNotice("s1", port.NoticeAutomation))
	}
	stores := []port.StoreID{"s1"}
	first, _ := b.List(t.Context(), port.NoticeQuery{Stores: stores, Limit: 3})
	if len(first) != 3 || first[0].ID < first[1].ID || first[1].ID < first[2].ID {
		t.Fatalf("first = %+v", first)
	}
	last := first[2]
	rest, _ := b.List(t.Context(), port.NoticeQuery{Stores: stores, Limit: 3, Before: &port.NoticeCursor{At: last.At, ID: last.ID}})
	if len(rest) != 2 || rest[0].ID >= last.ID {
		t.Errorf("rest = %+v", rest)
	}
}

func TestFakeInboxDefaultClockAdvances(t *testing.T) {
	b := &Inbox{}
	_ = b.Add(t.Context(), fakeNotice("s1", port.NoticeAutomation))
	_ = b.Add(t.Context(), fakeNotice("s1", port.NoticeAutomation))
	rows := fakeList(t, b, "s1")
	if !rows[0].At.After(rows[1].At) || !rows[1].At.Equal(time.Unix(0, 0)) {
		t.Errorf("at = %v, %v", rows[0].At, rows[1].At)
	}
}

func TestFakeInboxUnreadAndRead(t *testing.T) {
	b := &Inbox{}
	_ = b.Add(t.Context(), fakeNotice("s1", port.NoticeAutomation))
	_ = b.Add(t.Context(), fakeNotice("s1", port.NoticeAutomation))
	_ = b.Add(t.Context(), fakeNotice("s1", port.NoticeAutomationStopped))
	stores := []port.StoreID{"s1"}
	want := map[port.NoticeKind]int{port.NoticeAutomation: 2, port.NoticeAutomationStopped: 1}
	if got, _ := b.Unread(t.Context(), stores); !maps.Equal(got, want) {
		t.Fatalf("unread = %v", got)
	}
	id := fakeList(t, b, "s1")[0].ID
	_ = b.SetRead(t.Context(), id, true)
	if got, _ := b.Unread(t.Context(), stores); got[port.NoticeAutomationStopped] != 0 {
		t.Errorf("after read = %v", got)
	}
	_ = b.SetRead(t.Context(), id, false)
	if got, _ := b.Unread(t.Context(), stores); !maps.Equal(got, want) {
		t.Errorf("after unread = %v", got)
	}
	_ = b.ReadAll(t.Context(), stores, []port.NoticeKind{port.NoticeAutomation})
	if got, _ := b.Unread(t.Context(), stores); got[port.NoticeAutomation] != 0 || got[port.NoticeAutomationStopped] != 1 {
		t.Errorf("after ReadAll = %v", got)
	}
	if err := b.SetRead(t.Context(), 999, true); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("SetRead missing = %v", err)
	}
}

func TestFakeInboxResolve(t *testing.T) {
	b := &Inbox{}
	_ = b.Add(t.Context(), fakeNotice("s1", port.NoticeAutomationStopped))
	id := fakeList(t, b, "s1")[0].ID
	if err := b.Resolve(t.Context(), id, ""); !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("resolve empty = %v", err)
	}
	if got, _ := b.Get(t.Context(), id); got.Read || got.Result != "" {
		t.Errorf("after invalid resolve = %+v", got)
	}
	if err := b.Resolve(t.Context(), id, port.NoticeApplied); err != nil {
		t.Fatal(err)
	}
	got, _ := b.Get(t.Context(), id)
	if !got.Read || got.Result != port.NoticeApplied {
		t.Fatalf("get = %+v", got)
	}
	var r *refusal.Refusal
	if err := b.Resolve(t.Context(), id, port.NoticeKept); !errors.As(err, &r) || r.Message != "この通知はすでに処理されています" {
		t.Errorf("second resolve = %v", err)
	}
	if err := b.Resolve(t.Context(), 999, port.NoticeApplied); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("resolve missing = %v", err)
	}
}

func TestFakeInboxPrune(t *testing.T) {
	b := &Inbox{}
	byWorkspace := fakeNotice("s1", port.NoticeAutomation)
	byWorkspace.Item = ""
	other := fakeNotice("s1", port.NoticeAssign)
	other.Item, other.Workspace = "i9", "w9"
	for _, n := range []port.Notice{fakeNotice("s1", port.NoticeMention), byWorkspace, other, fakeNotice("s2", port.NoticeMention)} {
		_ = b.Add(t.Context(), n)
	}
	_ = b.Prune(t.Context(), "s1", []change.ID{"i1"})
	if got := fakeList(t, b, "s1"); len(got) != 2 {
		t.Fatalf("after item prune = %d", len(got))
	}
	_ = b.Prune(t.Context(), "s1", []change.ID{"w1"})
	if got := fakeList(t, b, "s1"); len(got) != 1 || got[0].Kind != port.NoticeAssign {
		t.Fatalf("after workspace prune = %+v", got)
	}
	if got := fakeList(t, b, "s2"); len(got) != 1 {
		t.Errorf("s2 = %d", len(got))
	}
}
