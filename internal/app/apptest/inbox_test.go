package apptest

import (
	"errors"
	"maps"
	"slices"
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
	// 移ったアイテムの通知は、元のワークスペースを持ったまま残る。ワークスペースの削除では消さない。
	moved := fakeNotice("s1", port.NoticeMention)
	moved.Item = "i8"
	for _, n := range []port.Notice{fakeNotice("s1", port.NoticeMention), byWorkspace, other, fakeNotice("s2", port.NoticeMention), moved} {
		_ = b.Add(t.Context(), n)
	}
	_ = b.Prune(t.Context(), "s1", []change.ID{"i1"})
	if got := fakeList(t, b, "s1"); len(got) != 3 {
		t.Fatalf("after item prune = %d", len(got))
	}
	_ = b.Prune(t.Context(), "s1", []change.ID{"w1"})
	if got := fakeList(t, b, "s1"); len(got) != 2 || !slices.ContainsFunc(got, func(r port.NoticeRow) bool { return r.Item == "i8" }) {
		t.Fatalf("after workspace prune = %+v", got)
	}
	if got := fakeList(t, b, "s2"); len(got) != 1 {
		t.Errorf("s2 = %d", len(got))
	}
}

func TestFakeInboxMentionUpsert(t *testing.T) {
	b := &Inbox{}
	at9, at930 := time.Unix(9*3600, 0), time.Unix(9*3600+1800, 0)
	first := port.Notice{Kind: port.NoticeMention, Store: "s1", Item: "i1", Source: "comment:c1", Text: "見て", Sender: "taro", At: at9}
	_ = b.Mention(t.Context(), first)
	rows := fakeList(t, b, "s1")
	if len(rows) != 1 || !rows[0].At.Equal(at9) {
		t.Fatalf("rows = %+v", rows)
	}
	_ = b.SetRead(t.Context(), rows[0].ID, true)
	second := first
	second.Text, second.Sender, second.At = "直した", "jiro", at930
	_ = b.Mention(t.Context(), second)
	other := first
	other.Store = "s2"
	_ = b.Mention(t.Context(), other)
	rows = fakeList(t, b, "s1")
	if len(rows) != 1 || rows[0].Text != "直した" || rows[0].Sender != "jiro" || !rows[0].At.Equal(at930) || rows[0].Read {
		t.Fatalf("rows = %+v", rows)
	}
	if len(fakeList(t, b, "s2")) != 1 {
		t.Error("another store must keep its own row")
	}
	if err := b.Mention(t.Context(), port.Notice{Kind: port.NoticeMention, Store: "s1"}); err == nil {
		t.Error("an empty Source must be an error")
	}
}

func TestFakeInboxProjectColumnsAndPrune(t *testing.T) {
	b := &Inbox{}
	byProject := port.Notice{Kind: port.NoticeAssign, Store: "s1", Workspace: "w1", Project: "p1", Display: "HH-P1"}
	itemSameID := fakeNotice("s1", port.NoticeMention)
	itemSameID.Item, itemSameID.Workspace = "i1", "p1"
	_ = b.Add(t.Context(), byProject)
	_ = b.Add(t.Context(), itemSameID)
	rows := fakeList(t, b, "s1")
	if rows[1].Project != "p1" || rows[1].Display != "HH-P1" {
		t.Fatalf("rows = %+v", rows)
	}
	_ = b.Prune(t.Context(), "s1", []change.ID{"p1"})
	if got := fakeList(t, b, "s1"); len(got) != 1 || got[0].Item != "i1" {
		t.Errorf("after prune = %+v", got)
	}
}
