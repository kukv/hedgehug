package inbox_test

import (
	"context"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/inbox"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

func (f fixture) write(t *testing.T, fn func(tx port.Tx) error) {
	t.Helper()
	if err := f.stores.Write(t.Context(), s1, port.Operation{Actor: change.User("taro"), Summary: "テスト"}, fn); err != nil {
		t.Fatal(err)
	}
}

func (f fixture) first(t *testing.T) inbox.Row {
	t.Helper()
	p := f.page(t, inbox.TabAll, nil)
	if len(p.Rows) == 0 {
		t.Fatal("no rows")
	}
	return p.Rows[0]
}

func TestTargetItem(t *testing.T) {
	f := setup(t)
	f.add(t, changed("1"))
	got := f.first(t).Target
	want := inbox.Target{
		Kind: inbox.TargetItem, Item: apptest.SampleTask, ItemKind: item.Task, Display: "HH-2", Title: "ログイン画面を作る",
		Workspace: apptest.SampleWorkspace, WorkspaceName: "Web",
	}
	if got != want {
		t.Errorf("target = %+v, want %+v", got, want)
	}
	f.write(t, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewSet("prefix", "HH", "WEB"))
	})
	if got := f.first(t).Target.Display; got != "WEB-2" {
		t.Errorf("display after the prefix change = %q, want WEB-2", got)
	}
}

func TestTargetDeletedItem(t *testing.T) {
	f := setup(t)
	f.add(t, changed("1"))
	if err := apptest.DeleteItems(t.Context(), f.stores, s1, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if got := f.first(t).Target; got.Kind != inbox.TargetDeletedItem {
		t.Errorf("target = %+v, want deleted_item", got)
	}
}

func TestTargetMoved(t *testing.T) {
	f := setup(t)
	n := changed("1")
	n.Item, n.Display = "01SAMPLEIT0000000000000009", "HH-9"
	f.add(t, n)
	if got := f.first(t).Target; got.Kind != inbox.TargetMoved || got.Display != "HH-9" || got.Item != n.Item {
		t.Errorf("target = %+v, want moved HH-9", got)
	}
}

func TestTargetRuleNotice(t *testing.T) {
	f := setup(t)
	f.add(t, port.Notice{Kind: port.NoticeAutomation, Store: s1, Workspace: apptest.SampleWorkspace, Rule: rule, Text: "rule"})
	got := f.first(t).Target
	want := inbox.Target{Kind: inbox.TargetWorkspace, Workspace: apptest.SampleWorkspace, WorkspaceName: "Web", Section: "automation"}
	if got != want {
		t.Errorf("target = %+v, want %+v", got, want)
	}
	if err := apptest.DeleteWorkspace(t.Context(), f.stores, s1, apptest.SampleWorkspace); err != nil {
		t.Fatal(err)
	}
	if got := f.first(t).Target; got.Kind != inbox.TargetDeletedWorkspace {
		t.Errorf("target = %+v, want deleted_workspace", got)
	}
}

func TestActionReadOnlyBeforeDeletedItem(t *testing.T) {
	f := setup(t)
	f.add(t, stopped())
	f.list.Stores[0].ReadOnly = true
	if err := apptest.DeleteItems(t.Context(), f.stores, s1, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	want := inbox.Action{Status: apptest.SampleStatusDone, StatusName: "Done", Visible: true, Note: "閲覧のみの Store です"}
	if a := f.first(t).Action; a == nil || *a != want {
		t.Errorf("action = %+v, want %+v", a, want)
	}
}

func TestActionHiddenWhenItemMoved(t *testing.T) {
	f := setup(t)
	n := stopped()
	n.Item, n.Display = "01SAMPLEIT0000000000000009", "HH-9"
	f.add(t, n)
	r := f.first(t)
	if r.Target.Kind != inbox.TargetMoved || r.Action == nil || r.Action.Visible {
		t.Errorf("target = %+v, action = %+v, want moved and not visible", r.Target, r.Action)
	}
}

func TestActionStates(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(t *testing.T, f fixture)
		want    inbox.Action
	}{
		{
			name:    "閲覧のみの Store",
			prepare: func(_ *testing.T, f fixture) { f.list.Stores[0].ReadOnly = true },
			want:    inbox.Action{Visible: true, Note: "閲覧のみの Store です"},
		},
		{
			name: "アイテムが削除済み",
			prepare: func(t *testing.T, f fixture) {
				if err := apptest.DeleteItems(context.Background(), f.stores, s1, apptest.SampleTask); err != nil {
					t.Fatal(err)
				}
			},
			want: inbox.Action{Visible: false},
		},
		{
			name: "いまのステータスが Cancel",
			prepare: func(t *testing.T, f fixture) {
				f.write(t, func(tx port.Tx) error {
					return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusCancel))
				})
			},
			want: inbox.Action{Visible: true, KeepEnabled: true, Note: "すでに変更されています(いまは Cancel)"},
		},
		{
			name: "変える先が削除済み",
			prepare: func(t *testing.T, f fixture) {
				f.write(t, func(tx port.Tx) error {
					return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewDelete("statuses/"+string(apptest.SampleStatusDone)))
				})
			},
			want: inbox.Action{Visible: true, KeepEnabled: true, Note: "「Done」は削除されています"},
		},
		{
			name:    "どれでもない",
			prepare: func(*testing.T, fixture) {},
			want:    inbox.Action{Visible: true, ApplyEnabled: true, KeepEnabled: true},
		},
		{
			name: "変更しないを選んだあと",
			prepare: func(t *testing.T, f fixture) {
				if err := f.book.Resolve(context.Background(), 1, port.NoticeKept); err != nil {
					t.Fatal(err)
				}
			},
			want: inbox.Action{Visible: true, ApplyEnabled: true, KeepEnabled: true, Result: port.NoticeKept},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setup(t)
			f.add(t, stopped())
			c.prepare(t, f)
			r := f.first(t)
			if r.Action == nil {
				t.Fatal("Action = nil")
			}
			want := c.want
			want.Status, want.StatusName = apptest.SampleStatusDone, "Done"
			if *r.Action != want {
				t.Errorf("action = %+v, want %+v", *r.Action, want)
			}
			if r.Reason != "ワークフロー" {
				t.Errorf("reason = %q", r.Reason)
			}
		})
	}
}
