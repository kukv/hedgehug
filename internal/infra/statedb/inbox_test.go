package statedb

import (
	"errors"
	"maps"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

var inboxT0 = time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)

func inboxNotice(store port.StoreID, kind port.NoticeKind) port.Notice {
	return port.Notice{Kind: kind, Store: store, Workspace: "w1", Item: "i1", Rule: "r1", Text: "本文", Reason: "理由", Status: "st1", From: "st0", Display: "HH-1", Sender: "alice"}
}

func inboxList(t *testing.T, b port.InboxBook, stores ...port.StoreID) []port.NoticeRow {
	t.Helper()
	rows, err := b.List(t.Context(), port.NoticeQuery{Stores: stores, Limit: 300})
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestInboxAddAndList(t *testing.T) {
	clock := apptest.NewClock(inboxT0)
	clock.Step = time.Minute
	b := openTemp(t).Inbox(clock)
	for range 3 {
		if err := b.Add(t.Context(), inboxNotice("s1", port.NoticeAutomationStopped)); err != nil {
			t.Fatal(err)
		}
	}
	rows := inboxList(t, b, "s1")
	if len(rows) != 3 {
		t.Fatalf("rows = %d", len(rows))
	}
	for i, r := range rows {
		want := inboxT0.Add(time.Duration(2-i) * time.Minute)
		if !r.At.Equal(want) || r.Read {
			t.Errorf("row %d: at=%v read=%v", i, r.At, r.Read)
		}
		if r.Notice != inboxNotice("s1", port.NoticeAutomationStopped) {
			t.Errorf("row %d: notice = %+v", i, r.Notice)
		}
	}
}

func TestInboxListPagesBySameTime(t *testing.T) {
	clock := apptest.NewClock(inboxT0)
	clock.Step = 0
	b := openTemp(t).Inbox(clock)
	for range 5 {
		if err := b.Add(t.Context(), inboxNotice("s1", port.NoticeAutomation)); err != nil {
			t.Fatal(err)
		}
	}
	stores := []port.StoreID{"s1"}
	first, err := b.List(t.Context(), port.NoticeQuery{Stores: stores, Limit: 3})
	if err != nil || len(first) != 3 {
		t.Fatalf("first = %d, %v", len(first), err)
	}
	if first[0].ID < first[1].ID || first[1].ID < first[2].ID {
		t.Errorf("not descending: %v %v %v", first[0].ID, first[1].ID, first[2].ID)
	}
	last := first[2]
	rest, err := b.List(t.Context(), port.NoticeQuery{Stores: stores, Limit: 3, Before: &port.NoticeCursor{At: last.At, ID: last.ID}})
	if err != nil || len(rest) != 2 {
		t.Fatalf("rest = %d, %v", len(rest), err)
	}
	seen := map[port.NoticeID]bool{}
	for _, r := range append(first, rest...) {
		seen[r.ID] = true
	}
	if len(seen) != 5 {
		t.Errorf("overlap: %d distinct", len(seen))
	}
}

func TestInboxListFiltersStoresAndKinds(t *testing.T) {
	b := openTemp(t).Inbox(apptest.NewClock(inboxT0))
	_ = b.Add(t.Context(), inboxNotice("s1", port.NoticeAutomation))
	_ = b.Add(t.Context(), inboxNotice("s1", port.NoticeAutomationStopped))
	_ = b.Add(t.Context(), inboxNotice("s2", port.NoticeAutomation))
	if got := inboxList(t, b, "s1"); len(got) != 2 {
		t.Errorf("s1 = %d", len(got))
	}
	if got := inboxList(t, b); len(got) != 0 {
		t.Errorf("no stores = %d", len(got))
	}
	rows, err := b.List(t.Context(), port.NoticeQuery{Stores: []port.StoreID{"s1", "s2"}, Kinds: []port.NoticeKind{port.NoticeAutomationStopped}, Limit: 300})
	if err != nil || len(rows) != 1 || rows[0].Kind != port.NoticeAutomationStopped {
		t.Errorf("kinds = %+v, %v", rows, err)
	}
}

func TestInboxUnreadAndRead(t *testing.T) {
	b := openTemp(t).Inbox(apptest.NewClock(inboxT0))
	_ = b.Add(t.Context(), inboxNotice("s1", port.NoticeAutomation))
	_ = b.Add(t.Context(), inboxNotice("s1", port.NoticeAutomation))
	_ = b.Add(t.Context(), inboxNotice("s1", port.NoticeAutomationStopped))
	unread := func() map[port.NoticeKind]int {
		m, err := b.Unread(t.Context(), []port.StoreID{"s1"})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	want := map[port.NoticeKind]int{port.NoticeAutomation: 2, port.NoticeAutomationStopped: 1}
	if got := unread(); !maps.Equal(got, want) {
		t.Fatalf("unread = %v", got)
	}
	id := inboxList(t, b, "s1")[0].ID // 新しい順の先頭は止まった通知
	if err := b.SetRead(t.Context(), id, true); err != nil {
		t.Fatal(err)
	}
	if got := unread(); got[port.NoticeAutomationStopped] != 0 || got[port.NoticeAutomation] != 2 {
		t.Errorf("after read = %v", got)
	}
	_ = b.SetRead(t.Context(), id, false)
	if got := unread(); !maps.Equal(got, want) {
		t.Errorf("after unread = %v", got)
	}
	if err := b.ReadAll(t.Context(), []port.StoreID{"s1"}, []port.NoticeKind{port.NoticeAutomation}); err != nil {
		t.Fatal(err)
	}
	if got := unread(); got[port.NoticeAutomation] != 0 || got[port.NoticeAutomationStopped] != 1 {
		t.Errorf("after ReadAll = %v", got)
	}
	if err := b.SetRead(t.Context(), 999, true); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("SetRead missing = %v", err)
	}
}

func TestInboxResolve(t *testing.T) {
	b := openTemp(t).Inbox(apptest.NewClock(inboxT0))
	_ = b.Add(t.Context(), inboxNotice("s1", port.NoticeAutomationStopped))
	id := inboxList(t, b, "s1")[0].ID
	if err := b.Resolve(t.Context(), id, ""); !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("resolve empty = %v", err)
	}
	if got, _ := b.Get(t.Context(), id); got.Read || got.Result != "" {
		t.Errorf("after invalid resolve = %+v", got)
	}
	if err := b.Resolve(t.Context(), id, port.NoticeApplied); err != nil {
		t.Fatal(err)
	}
	got, err := b.Get(t.Context(), id)
	if err != nil || !got.Read || got.Result != port.NoticeApplied {
		t.Fatalf("get = %+v, %v", got, err)
	}
	var r *refusal.Refusal
	if err := b.Resolve(t.Context(), id, port.NoticeKept); !errors.As(err, &r) || r.Message != "この通知はすでに処理されています" {
		t.Errorf("second resolve = %v", err)
	}
	if err := b.Resolve(t.Context(), 999, port.NoticeApplied); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("resolve missing = %v", err)
	}
	if _, err := b.Get(t.Context(), 999); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("get missing = %v", err)
	}
}

func TestInboxOrdersSubSecond(t *testing.T) {
	clock := apptest.NewClock(inboxT0)
	clock.Step = 500 * time.Millisecond
	b := openTemp(t).Inbox(clock)
	_ = b.Add(t.Context(), inboxNotice("s1", port.NoticeAutomation)) // 10:00:00.000
	_ = b.Add(t.Context(), inboxNotice("s1", port.NoticeAutomation)) // 10:00:00.500
	rows := inboxList(t, b, "s1")
	if len(rows) != 2 || !rows[0].At.Equal(inboxT0.Add(500*time.Millisecond)) {
		t.Fatalf("rows = %+v", rows)
	}
	before, err := b.List(t.Context(), port.NoticeQuery{Stores: []port.StoreID{"s1"}, Limit: 300, Before: &port.NoticeCursor{At: rows[0].At, ID: rows[0].ID}})
	if err != nil || len(before) != 1 || !before[0].At.Equal(inboxT0) {
		t.Errorf("before = %+v, %v", before, err)
	}
}

func TestInboxPrune(t *testing.T) {
	b := openTemp(t).Inbox(apptest.NewClock(inboxT0))
	byItem := inboxNotice("s1", port.NoticeMention)
	byWorkspace := inboxNotice("s1", port.NoticeAutomation)
	byWorkspace.Item = ""
	other := inboxNotice("s1", port.NoticeAssign)
	other.Item, other.Workspace = "i9", "w9"
	otherStore := inboxNotice("s2", port.NoticeMention)
	// 移ったアイテムの通知は、元のワークスペースを持ったまま残る。ワークスペースの削除では消さない。
	moved := inboxNotice("s1", port.NoticeMention)
	moved.Item = "i8"
	for _, n := range []port.Notice{byItem, byWorkspace, other, otherStore, moved} {
		if err := b.Add(t.Context(), n); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.Prune(t.Context(), "s1", []change.ID{"i1"}); err != nil {
		t.Fatal(err)
	}
	if got := inboxList(t, b, "s1"); len(got) != 3 {
		t.Fatalf("after item prune = %d", len(got))
	}
	if err := b.Prune(t.Context(), "s1", []change.ID{"w1"}); err != nil {
		t.Fatal(err)
	}
	if got := inboxList(t, b, "s1"); len(got) != 2 || !slices.ContainsFunc(got, func(r port.NoticeRow) bool { return r.Item == "i8" }) {
		t.Fatalf("after workspace prune = %+v", got)
	}
	if got := inboxList(t, b, "s2"); len(got) != 1 {
		t.Errorf("s2 = %d", len(got))
	}
}

func TestInboxForgetStore(t *testing.T) {
	b := openTemp(t).Inbox(apptest.NewClock(inboxT0))
	_ = b.Add(t.Context(), inboxNotice("s1", port.NoticeAutomation))
	_ = b.Add(t.Context(), inboxNotice("s2", port.NoticeAutomation))
	if err := b.ForgetStore(t.Context(), "s1"); err != nil {
		t.Fatal(err)
	}
	if len(inboxList(t, b, "s1")) != 0 || len(inboxList(t, b, "s2")) != 1 {
		t.Error("ForgetStore removed the wrong rows")
	}
}

func TestInboxSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	b := d.Inbox(apptest.NewClock(inboxT0))
	_ = b.Add(t.Context(), inboxNotice("s1", port.NoticeAutomation))
	_ = b.SetRead(t.Context(), inboxList(t, b, "s1")[0].ID, true)
	_ = d.Close()
	d, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	rows := inboxList(t, d.Inbox(apptest.NewClock(inboxT0)), "s1")
	if len(rows) != 1 || !rows[0].Read {
		t.Errorf("after reopen: %+v", rows)
	}
}

func TestInboxMentionUpsert(t *testing.T) {
	b := openTemp(t).Inbox(apptest.NewClock(inboxT0))
	at9, at930 := inboxT0.Add(-time.Hour), inboxT0.Add(-30*time.Minute)
	first := port.Notice{Kind: port.NoticeMention, Store: "s1", Workspace: "w1", Item: "i1", Source: "comment:c1", Text: "@hanako 見て", Sender: "taro", At: at9}
	if err := b.Mention(t.Context(), first); err != nil {
		t.Fatal(err)
	}
	rows := inboxList(t, b, "s1")
	if len(rows) != 1 || !rows[0].At.Equal(at9) || rows[0].Source != "comment:c1" {
		t.Fatalf("rows = %+v", rows)
	}
	if err := b.SetRead(t.Context(), rows[0].ID, true); err != nil {
		t.Fatal(err)
	}
	second := first
	second.Text, second.Sender, second.At = "@hanako 直した", "jiro", at930
	if err := b.Mention(t.Context(), second); err != nil {
		t.Fatal(err)
	}
	other := first
	other.Store = "s2"
	if err := b.Mention(t.Context(), other); err != nil {
		t.Fatal(err)
	}
	rows = inboxList(t, b, "s1")
	if len(rows) != 1 || rows[0].Text != "@hanako 直した" || rows[0].Sender != "jiro" || !rows[0].At.Equal(at930) || rows[0].Read || rows[0].Result != "" {
		t.Fatalf("rows = %+v", rows)
	}
	if got := inboxList(t, b, "s2"); len(got) != 1 || got[0].Text != "@hanako 見て" {
		t.Errorf("s2 = %+v", got)
	}
	if err := b.Mention(t.Context(), port.Notice{Kind: port.NoticeMention, Store: "s1"}); err == nil {
		t.Error("an empty Source must be an error")
	}
}

func TestInboxProjectColumns(t *testing.T) {
	b := openTemp(t).Inbox(apptest.NewClock(inboxT0))
	n := port.Notice{Kind: port.NoticeAssign, Store: "s1", Workspace: "w1", Project: "p1", Display: "HH-P1"}
	if err := b.Add(t.Context(), n); err != nil {
		t.Fatal(err)
	}
	rows := inboxList(t, b, "s1")
	if len(rows) != 1 || rows[0].Project != "p1" || rows[0].Display != "HH-P1" {
		t.Fatalf("rows = %+v", rows)
	}
	got, err := b.Get(t.Context(), rows[0].ID)
	if err != nil || got.Project != "p1" {
		t.Errorf("get = %+v, %v", got, err)
	}
}

func TestInboxPruneProject(t *testing.T) {
	b := openTemp(t).Inbox(apptest.NewClock(inboxT0))
	byProject := port.Notice{Kind: port.NoticeAssign, Store: "s1", Workspace: "w1", Project: "p1"}
	otherProject := byProject
	otherProject.Project = "p2"
	itemSameID := inboxNotice("s1", port.NoticeMention)
	itemSameID.Item, itemSameID.Workspace = "i1", "p1"
	for _, n := range []port.Notice{byProject, otherProject, itemSameID} {
		if err := b.Add(t.Context(), n); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.Prune(t.Context(), "s1", []change.ID{"p1"}); err != nil {
		t.Fatal(err)
	}
	got := inboxList(t, b, "s1")
	if len(got) != 2 || slices.ContainsFunc(got, func(r port.NoticeRow) bool { return r.Project == "p1" }) {
		t.Fatalf("after prune = %+v", got)
	}
	if !slices.ContainsFunc(got, func(r port.NoticeRow) bool { return r.Item == "i1" }) {
		t.Error("an item notice must stay")
	}
}
