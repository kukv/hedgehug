package inbox_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	checkProperty change.ID = "01SAMPLEPROP00000000000002"
	systemAPI     change.ID = "01SAMPLEVAL000000000000012"
	sprintS       change.ID = "01SAMPLESPRINT000000000001"
	otherTask     item.ID   = "01SAMPLEIT0000000000000005"
)

// recorder は、保存の操作(実行者・アクティビティの文)を覚える Stores。
type recorder struct {
	port.Stores
	ops []port.Operation
}

func (r *recorder) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	r.ops = append(r.ops, op)
	return r.Stores.Write(ctx, id, op, fn)
}

// reverter は、保存を覚える Stores と、出来事を覚える Events を付けた受信箱。
type reverter struct {
	fixture
	rec    *recorder
	events *apptest.Events
}

func withRevert(t *testing.T) reverter {
	t.Helper()
	f := setup(t)
	rec := &recorder{Stores: f.stores}
	ev := &apptest.Events{}
	f.svc.Stores, f.svc.Events = rec, ev
	return reverter{fixture: f, rec: rec, events: ev}
}

// notice は、通知を足して ID を返す。
func (r reverter) notice(t *testing.T, n port.Notice) port.NoticeID {
	t.Helper()
	r.add(t, n)
	rows, err := r.book.List(t.Context(), port.NoticeQuery{Stores: []port.StoreID{s1}, Limit: 10})
	if err != nil || len(rows) == 0 {
		t.Fatalf("list: %v %v", rows, err)
	}
	return rows[0].ID
}

// logOf は、Store s1 の ref の変更記録。
func (r reverter) logOf(t *testing.T, ref change.Ref) []change.Record {
	t.Helper()
	var recs []change.Record
	err := r.stores.Read(t.Context(), s1, func(rd port.Reader) error {
		var err error
		recs, err = rd.Changes().Of(ref)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return recs
}

// lastOp は、最後の保存(Op が同じ行)の行を「種類 項目 from to」の文字にしたもの。
func lastOp(recs []change.Record) []string {
	out := []string{}
	for _, r := range recs {
		if r.Op == recs[len(recs)-1].Op {
			out = append(out, string(r.T)+" "+r.F+" "+string(r.From)+" "+string(r.To))
		}
	}
	return out
}

func (r reverter) row(t *testing.T, id port.NoticeID) port.NoticeRow {
	t.Helper()
	row, err := r.book.Get(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	return row
}

// wantApplied は、保存できて、通知が既読・applied になり、出来事 inbox が 1 回出たことを確かめる。
func (r reverter) wantApplied(t *testing.T, id port.NoticeID) {
	t.Helper()
	if row := r.row(t, id); !row.Read || row.Result != port.NoticeApplied {
		t.Errorf("notice = %+v, want read and applied", row)
	}
	if ev := r.events.List(); len(ev) != 1 || ev[0] != (port.Event{Kind: port.EventInbox, Store: s1}) {
		t.Errorf("events = %v, want one inbox event", ev)
	}
}

// wantUntouched は、断ったあと、何も保存せず、通知が結果なしのままで、出来事も出ていないことを確かめる。
func (r reverter) wantUntouched(t *testing.T, id port.NoticeID, rows int) {
	t.Helper()
	if row := r.row(t, id); row.Result != "" || row.Read {
		t.Errorf("notice = %+v, want unresolved and unread", row)
	}
	if len(r.events.List()) != 0 {
		t.Errorf("events = %v, want none", r.events.List())
	}
	if n := len(r.logOf(t, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})); n != rows {
		t.Errorf("item rows = %d, want %d", n, rows)
	}
}

func TestRevertStatus(t *testing.T) {
	f := withRevert(t)
	f.setTask(t, change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusDone))
	id := f.notice(t, overwritten("status", apptest.SampleStatusTodo, apptest.SampleStatusDone))
	if err := f.svc.Revert(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	log := f.logOf(t, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
	want := `set status "` + string(apptest.SampleStatusDone) + `" "` + string(apptest.SampleStatusTodo) + `"`
	if got := lastOp(log); len(got) != 1 || got[0] != want || log[len(log)-1].Actor != change.User("taro") {
		t.Errorf("last op = %q (actor %+v), want %q by taro", got, log[len(log)-1].Actor, want)
	}
	if op := f.rec.ops[len(f.rec.ops)-1]; op.Summary != "HH-2 のステータスを上書き前の値に戻す" || op.Actor != change.User("taro") {
		t.Errorf("operation = %+v", op)
	}
	f.wantApplied(t, id)
}

func TestRevertSystems(t *testing.T) {
	f := withRevert(t)
	f.setTask(t, change.NewAdd("systems", apptest.SampleSystemWeb), change.NewAdd("systems", systemAPI))
	mine, kept := []change.ID{apptest.SampleSystemWeb}, []change.ID{systemAPI, apptest.SampleSystemWeb}
	id := f.notice(t, overwritten("systems", mine, kept))
	if err := f.svc.Revert(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	got := lastOp(f.logOf(t, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}))
	if len(got) != 1 || got[0] != `remove systems "`+string(systemAPI)+`" ` {
		t.Errorf("last op = %q, want remove systems API only", got)
	}
}

func TestRevertEmpty(t *testing.T) {
	check := item.FieldKey(checkProperty)
	cases := []struct {
		name       string
		field      string
		before     change.Draft
		mine, kept any
		project    bool
		want       string
	}{
		{"担当者", "assignee", change.NewSet("assignee", nil, "hanako"), nil, "hanako", false, `set assignee "hanako" null`},
		{"チェック", check, change.NewSet(check, nil, true), false, true, false, `set ` + check + ` true false`},
		{"プロジェクトのオーナー", "owner", change.NewSet("owner", "taro", "hanako"), "", "hanako", true, `set owner "hanako" ""`},
	}
	for _, c := range cases {
		f := withRevert(t)
		prop := workspace.Property{ID: checkProperty, Name: "確認済み", Type: workspace.TypeCheck, Kinds: workspace.Kinds, Rank: "z"}
		f.writeWorkspace(t, workspace.PropertyDrafts(prop)...)
		ref, n := change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}, overwritten(c.field, c.mine, c.kept)
		if c.project {
			ref, n = change.Ref{Kind: change.KindProject, ID: apptest.SampleProject}, projectOverwritten(c.field, c.mine, c.kept)
			f.write(t, func(tx port.Tx) error { return tx.WriteProjects().Append(apptest.SampleProject, c.before) })
		} else {
			f.setTask(t, c.before)
		}
		id := f.notice(t, n)
		if err := f.svc.Revert(t.Context(), id); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := lastOp(f.logOf(t, ref)); len(got) != 1 || got[0] != c.want {
			t.Errorf("%s: last op = %q, want %q", c.name, got, c.want)
		}
	}
}

// スプリントは、移動先の枠の末尾の rank も同じ保存で書く。
func TestRevertSprint(t *testing.T) {
	f := withRevert(t)
	f.writeWorkspace(t, planning.SprintDrafts(planning.Sprint{}, planning.Sprint{ID: sprintS, Name: "Sprint 1", State: planning.Planned})...)
	other := item.Item{
		ID: otherTask, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Number: 5},
		Kind: item.Task, Rank: "x", Title: "別", Status: apptest.SampleStatusTodo, Sprint: sprintS,
	}
	f.write(t, func(tx port.Tx) error { return tx.WriteItems().Append(other.ID, item.Create(other)) })
	id := f.notice(t, overwritten("sprint", sprintS, nil))
	if err := f.svc.Revert(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	got := lastOp(f.logOf(t, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}))
	if len(got) != 2 || got[0] != `set sprint null "`+string(sprintS)+`"` || !strings.HasPrefix(got[1], "set rank ") {
		t.Errorf("last op = %q, want set sprint and set rank", got)
	}
}

func TestRevertProjectTitle(t *testing.T) {
	f := withRevert(t)
	f.write(t, func(tx port.Tx) error {
		return tx.WriteProjects().Append(apptest.SampleProject, change.NewSet("title", "ログイン", "新しい  名前"))
	})
	id := f.notice(t, projectOverwritten("title", "ログイン", "新しい  名前"))
	if err := f.svc.Revert(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	if op := f.rec.ops[len(f.rec.ops)-1]; op.Summary != "新しい 名前 のタイトルを上書き前の値に戻す" {
		t.Errorf("summary = %q", op.Summary)
	}
	f.wantApplied(t, id)
}

func TestRevertChangedMeanwhile(t *testing.T) {
	f := withRevert(t)
	f.setTask(t, change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusDone))
	id := f.notice(t, overwritten("status", apptest.SampleStatusTodo, apptest.SampleStatusDone))
	f.setTask(t, change.NewSet("status", apptest.SampleStatusDone, apptest.SampleStatusInProgress))
	rows := len(f.logOf(t, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}))
	err := f.svc.Revert(t.Context(), id)
	if got := refusalText(err); got != "HH-2 は先に変更されたため、適用しませんでした" {
		t.Fatalf("err = %v, want changed meanwhile", err)
	}
	f.wantUntouched(t, id, rows)
}

func TestRevertRefusals(t *testing.T) {
	status := func() port.Notice { return overwritten("status", apptest.SampleStatusTodo, apptest.SampleStatusDone) }
	toDone := change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusDone)
	cases := []struct {
		name    string
		prepare func(f reverter, id port.NoticeID)
		notice  port.Notice
		text    string
		err     error
	}{
		{"閲覧のみ", func(f reverter, _ port.NoticeID) { f.list.Stores[0].ReadOnly = true }, status(), "閲覧のみの Store です", nil},
		{"結果があと", func(f reverter, id port.NoticeID) {
			if err := f.book.Resolve(t.Context(), id, port.NoticeKept); err != nil {
				t.Fatal(err)
			}
		}, status(), "この通知はすでに処理されています", nil},
		{"削除済みのアイテム", func(f reverter, _ port.NoticeID) {
			if err := apptest.DeleteItems(t.Context(), f.stores, s1, apptest.SampleTask); err != nil {
				t.Fatal(err)
			}
		}, status(), "HH-2 は先に変更されたため、適用しませんでした", nil},
		{"削除されたステータス", func(f reverter, _ port.NoticeID) {
			f.writeWorkspace(t, change.NewDelete("statuses/"+string(apptest.SampleStatusTodo)))
		}, status(), "「Todo」は削除されています", nil},
		{"Store にないアイテム", func(reverter, port.NoticeID) {}, func() port.Notice {
			n := status()
			n.Item = "01SAMPLEIT0000000000000009"
			return n
		}(), "HH-2 は先に変更されたため、適用しませんでした", nil},
		{"削除した追加プロパティ", func(f reverter, _ port.NoticeID) {
			prop := workspace.Property{ID: textProperty, Name: "メモ", Type: workspace.TypeText, Kinds: workspace.Kinds, Rank: "z"}
			f.writeWorkspace(t, workspace.PropertyDrafts(prop)...)
			f.writeWorkspace(t, change.NewDelete("properties/"+string(textProperty)))
		}, overwritten(item.FieldKey(textProperty), "a", "b"), "HH-2 は先に変更されたため、適用しませんでした", nil},
		{"本文だけの項目", func(reverter, port.NoticeID) {}, projectOverwritten("project_rules/x", "a", "b"), "", port.ErrInvalidOperation},
		{"上書きでない通知", func(reverter, port.NoticeID) {}, changed("自動化"), "", port.ErrInvalidOperation},
		{"表示中でない Store", func(f reverter, _ port.NoticeID) { f.list.Stores = f.list.Stores[1:] }, status(), "", port.ErrNotFound},
	}
	for _, c := range cases {
		f := withRevert(t)
		f.setTask(t, toDone)
		id := f.notice(t, c.notice)
		c.prepare(f, id)
		rows := len(f.logOf(t, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}))
		resultBefore := f.row(t, id).Result
		err := f.svc.Revert(t.Context(), id)
		switch {
		case c.err != nil && !errors.Is(err, c.err):
			t.Errorf("%s: err = %v, want %v", c.name, err, c.err)
		case c.err == nil && refusalText(err) != c.text:
			t.Errorf("%s: err = %v, want refusal %q", c.name, err, c.text)
		}
		if got := f.row(t, id).Result; got != resultBefore {
			t.Errorf("%s: result = %q, want %q (unchanged)", c.name, got, resultBefore)
		}
		if len(f.events.List()) != 0 {
			t.Errorf("%s: events = %v, want none", c.name, f.events.List())
		}
		if got := len(f.logOf(t, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})); got != rows {
			t.Errorf("%s: item rows = %d, want %d (nothing saved)", c.name, got, rows)
		}
	}
}
