package inbox_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/inbox"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// applyCall は、偽物の「制約を無視して…」に渡された引数。
type applyCall struct {
	store              port.StoreID
	id                 item.ID
	rule, from, status change.ID
}

// actor は、操作を足した受信箱。偽物の「制約を無視して…」は、Err が nil ならステータスを書く。
type actor struct {
	fixture
	events *apptest.Events
	calls  []applyCall
	Err    error
}

func withActions(t *testing.T) *actor {
	t.Helper()
	a := &actor{fixture: setup(t), events: &apptest.Events{}}
	a.svc.Events = a.events
	a.svc.ApplyIgnoringWorkflow = func(ctx context.Context, store port.StoreID, id item.ID, rule, from, status change.ID) error {
		a.calls = append(a.calls, applyCall{store, id, rule, from, status})
		if a.Err != nil {
			return a.Err
		}
		return a.stores.Write(ctx, store, port.Operation{Actor: change.User("hanako"), Summary: "適用"}, func(tx port.Tx) error {
			return tx.WriteItems().Append(id, change.NewSet("status", from, status))
		})
	}
	return a
}

func (a *actor) row(t *testing.T, id port.NoticeID) port.NoticeRow {
	t.Helper()
	r, err := a.book.Get(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (a *actor) status(t *testing.T) change.ID {
	t.Helper()
	var it item.Item
	_ = a.stores.Read(t.Context(), s1, func(r port.Reader) error {
		it, _ = r.Items().Get(apptest.SampleTask)
		return nil
	})
	return it.Status
}

func (a *actor) wantEvents(t *testing.T, stores ...port.StoreID) {
	t.Helper()
	want := []port.Event{}
	for _, s := range stores {
		want = append(want, port.Event{Kind: port.EventInbox, Store: s})
	}
	if got := a.events.List(); !slices.Equal(got, want) {
		t.Errorf("events = %v, want %v", got, want)
	}
}

func refusalText(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}

func TestSetReadPublishes(t *testing.T) {
	a := withActions(t)
	a.add(t, changed("1"))
	if err := a.svc.SetRead(t.Context(), 1, true); err != nil {
		t.Fatal(err)
	}
	if !a.row(t, 1).Read {
		t.Error("not read")
	}
	a.wantEvents(t, s1)
}

func TestReadAllOnlyTab(t *testing.T) {
	a := withActions(t)
	mention := port.Notice{Kind: port.NoticeMention, Store: s1, Workspace: apptest.SampleWorkspace, Item: apptest.SampleTask, Text: "m"}
	a.add(t, changed("1"), stopped(), mention)
	if err := a.svc.ReadAll(t.Context(), inbox.TabAutomation); err != nil {
		t.Fatal(err)
	}
	if !a.row(t, 1).Read || !a.row(t, 2).Read || a.row(t, 3).Read {
		t.Errorf("read = %v %v %v, want true true false", a.row(t, 1).Read, a.row(t, 2).Read, a.row(t, 3).Read)
	}
	a.wantEvents(t, "")
}

func TestReadAllSkipsHiddenStore(t *testing.T) {
	a := withActions(t)
	other := changed("2")
	other.Store = s2
	a.add(t, changed("1"), other)
	a.list.Stores = a.list.Stores[:1] // team を非表示にする
	if err := a.svc.ReadAll(t.Context(), inbox.TabAll); err != nil {
		t.Fatal(err)
	}
	if !a.row(t, 1).Read || a.row(t, 2).Read {
		t.Errorf("read = %v %v, want true false", a.row(t, 1).Read, a.row(t, 2).Read)
	}
}

func TestKeep(t *testing.T) {
	a := withActions(t)
	a.add(t, stopped())
	if err := a.svc.Keep(t.Context(), 1); err != nil {
		t.Fatal(err)
	}
	if r := a.row(t, 1); !r.Read || r.Result != port.NoticeKept {
		t.Errorf("row = %+v", r)
	}
	if len(a.calls) != 0 || a.status(t) != apptest.SampleStatusTodo {
		t.Error("keep must not save")
	}
	a.wantEvents(t, s1)
}

func TestApply(t *testing.T) {
	a := withActions(t)
	a.add(t, stopped())
	if err := a.svc.Apply(t.Context(), 1); err != nil {
		t.Fatal(err)
	}
	want := []applyCall{{s1, apptest.SampleTask, rule, apptest.SampleStatusTodo, apptest.SampleStatusDone}}
	if !slices.Equal(a.calls, want) {
		t.Errorf("calls = %v, want %v", a.calls, want)
	}
	if r := a.row(t, 1); !r.Read || r.Result != port.NoticeApplied {
		t.Errorf("row = %+v", r)
	}
	if got := a.status(t); got != apptest.SampleStatusDone {
		t.Errorf("status = %q, want Done", got)
	}
	a.wantEvents(t, s1)
}

func TestApplyAfterKeepRefused(t *testing.T) {
	a := withActions(t)
	a.add(t, stopped())
	if err := a.svc.Keep(t.Context(), 1); err != nil {
		t.Fatal(err)
	}
	err := a.svc.Apply(t.Context(), 1)
	if got := refusalText(err); got != "この通知はすでに処理されています" {
		t.Errorf("refusal = %q (err %v)", got, err)
	}
	if len(a.calls) != 0 || a.status(t) != apptest.SampleStatusTodo {
		t.Error("a handled notice must not be applied")
	}
	if r := a.row(t, 1); r.Result != port.NoticeKept {
		t.Errorf("result = %q, want kept", r.Result)
	}
	a.wantEvents(t, s1) // Keep の 1 回だけ
}

func TestApplyRefusedWhenStatusChanged(t *testing.T) {
	a := withActions(t)
	a.add(t, stopped())
	a.Err = refusal.New("HH-2 は先に変更されたため、適用しませんでした")
	err := a.svc.Apply(t.Context(), 1)
	if got := refusalText(err); got != "HH-2 は先に変更されたため、適用しませんでした" {
		t.Errorf("refusal = %q (err %v)", got, err)
	}
	if r := a.row(t, 1); r.Read || r.Result != "" {
		t.Errorf("row = %+v, want unread and unhandled", r)
	}
	a.wantEvents(t)
}

func TestApplyReadOnlyStore(t *testing.T) {
	a := withActions(t)
	a.add(t, stopped())
	a.list.Stores[0].ReadOnly = true
	err := a.svc.Apply(t.Context(), 1)
	if got := refusalText(err); got != "閲覧のみの Store です" {
		t.Errorf("refusal = %q (err %v)", got, err)
	}
	if len(a.calls) != 0 || a.row(t, 1).Result != "" {
		t.Error("a read-only Store must not be applied")
	}
	// 閲覧のみの Store では「変更しない」も無効(C4a-10 の (1))
	if got := refusalText(a.svc.Keep(t.Context(), 1)); got != "閲覧のみの Store です" {
		t.Errorf("keep refusal = %q", got)
	}
	if a.row(t, 1).Result != "" {
		t.Error("a read-only Store must not be kept")
	}
	a.wantEvents(t)
}

func TestApplyNotStopped(t *testing.T) {
	a := withActions(t)
	a.add(t, changed("1"))
	if err := a.svc.Apply(t.Context(), 1); !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("err = %v, want ErrInvalidOperation", err)
	}
	if err := a.svc.Keep(t.Context(), 1); !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("keep err = %v, want ErrInvalidOperation", err)
	}
	if len(a.calls) != 0 {
		t.Error("must not apply")
	}
}

func TestKeepApplyHiddenStore(t *testing.T) {
	a := withActions(t)
	a.add(t, stopped())
	a.list.Stores = a.list.Stores[1:] // web を非表示にする(止まった通知の Store)
	if err := a.svc.Keep(t.Context(), 1); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("keep err = %v, want ErrNotFound", err)
	}
	if err := a.svc.Apply(t.Context(), 1); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("apply err = %v, want ErrNotFound", err)
	}
	if len(a.calls) != 0 {
		t.Error("a hidden Store must not be applied")
	}
	if r := a.row(t, 1); r.Read || r.Result != "" {
		t.Errorf("row = %+v, want unread and unhandled", r)
	}
	a.wantEvents(t)
}

func TestNotifierPublishes(t *testing.T) {
	book := &apptest.Inbox{}
	events := &apptest.Events{}
	n := inbox.Notifier{Inbox: book, Events: events}
	if err := n.Add(t.Context(), changed("1")); err != nil {
		t.Fatal(err)
	}
	if len(book.Notices()) != 1 {
		t.Errorf("notices = %v", book.Notices())
	}
	if got := events.List(); !slices.Equal(got, []port.Event{{Kind: port.EventInbox, Store: s1}}) {
		t.Errorf("events = %v", got)
	}
	book.Err = errors.New("broken")
	if err := n.Add(t.Context(), changed("2")); err == nil {
		t.Error("want the error")
	}
	if len(events.List()) != 1 {
		t.Error("a failed Add must not publish")
	}
}

func TestNotifierMentionPublishes(t *testing.T) {
	book := &apptest.Inbox{}
	events := &apptest.Events{}
	n := inbox.Notifier{Inbox: book, Events: events}
	notice := port.Notice{Kind: port.NoticeMention, Store: s1, Source: "comment:c1", Text: "見て"}
	if err := n.Mention(t.Context(), notice); err != nil {
		t.Fatal(err)
	}
	if got := events.List(); !slices.Equal(got, []port.Event{{Kind: port.EventInbox, Store: s1}}) {
		t.Errorf("events = %v", got)
	}
	book.Err = errors.New("broken")
	if err := n.Mention(t.Context(), notice); err == nil {
		t.Error("want the error")
	}
	if len(events.List()) != 1 {
		t.Error("a failed Mention must not publish")
	}
}
