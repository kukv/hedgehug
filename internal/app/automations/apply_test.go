package automations_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
)

func TestApplyIgnoringWorkflow(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	r, _ := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	if err := svc.ApplyIgnoringWorkflow(ctx, store, apptest.SampleTask, r.ID, apptest.SampleStatusTodo, apptest.SampleStatusDone); err != nil {
		t.Fatal(err)
	}
	if last := rec.summaries[len(rec.summaries)-1]; last != "「完了で外す」の変更を制約を無視して適用" {
		t.Errorf("summary = %q", last)
	}
	var recs []change.Record
	_ = rec.Read(ctx, store, func(rd port.Reader) error {
		recs, _ = rd.Changes().Of(change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
		return nil
	})
	got := recs[len(recs)-1]
	want := change.Actor{Kind: change.ActorAutomation, Login: "hanako", Rule: r.ID}
	if got.F != "status" || got.Actor != want || string(got.To) != `"`+string(apptest.SampleStatusDone)+`"` {
		t.Errorf("row = %+v", got) // 自動化が行った変更、by は押した人(B4-11)
	}
	n := len(recs)
	_ = svc.ApplyIgnoringWorkflow(ctx, store, apptest.SampleTask, r.ID, apptest.SampleStatusDone, apptest.SampleStatusDone) // すでに Done: 何も保存しない
	_ = rec.Read(ctx, store, func(rd port.Reader) error {
		recs, _ = rd.Changes().Of(change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
		return nil
	})
	if len(recs) != n {
		t.Error("the same status must not be saved")
	}
	if err := svc.ApplyIgnoringWorkflow(ctx, store, apptest.SampleTask, "01NORULE", apptest.SampleStatusDone, apptest.SampleStatusDone); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("no rule: %v", err)
	}
	if err := svc.ApplyIgnoringWorkflow(ctx, store, apptest.SampleTask, r.ID, apptest.SampleStatusDone, "01NOSTATUS"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("no status: %v", err)
	}
}

// 使わないステータスにも「制約を無視して…」で変えられる(判定を通らない。B5a-7)。
func TestApplyIgnoringWorkflowToAHiddenStatus(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	r, _ := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	err := rec.Stores.Write(ctx, store, port.Operation{Actor: change.User("hanako"), Summary: "プロジェクト別のルールを変更"}, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewAdd(projectrule.HiddenField(apptest.SampleProject), apptest.SampleStatusPending))
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyIgnoringWorkflow(ctx, store, apptest.SampleTask, r.ID, apptest.SampleStatusTodo, apptest.SampleStatusPending); err != nil {
		t.Fatalf("ApplyIgnoringWorkflow to a hidden status = %v", err)
	}
	var it item.Item
	_ = rec.Read(ctx, store, func(rd port.Reader) error {
		it, _ = rd.Items().Get(apptest.SampleTask)
		return nil
	})
	if it.Status != apptest.SampleStatusPending {
		t.Errorf("status = %q, want Pending", it.Status)
	}
}

// 押す前にステータスが変わっていたら、保存せずに断る(C4a-11)。
func TestApplyRefusedWhenStatusChanged(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	r, _ := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	err := rec.Stores.Write(ctx, store, port.Operation{Actor: change.User("taro"), Summary: "ステータスを変更"}, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleEpic, change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusInProgress))
	})
	if err != nil {
		t.Fatal(err)
	}
	count := func() int {
		var recs []change.Record
		_ = rec.Read(ctx, store, func(rd port.Reader) error {
			recs, _ = rd.Changes().Of(change.Ref{Kind: change.KindItem, ID: apptest.SampleEpic})
			return nil
		})
		return len(recs)
	}
	n := count()
	err = svc.ApplyIgnoringWorkflow(ctx, store, apptest.SampleEpic, r.ID, apptest.SampleStatusTodo, apptest.SampleStatusDone)
	if got := message(err); got != "HH-1 は先に変更されたため、適用しませんでした" {
		t.Errorf("refusal = %q (err %v)", got, err)
	}
	if count() != n {
		t.Error("a refused apply must not be saved")
	}
	if err := svc.ApplyIgnoringWorkflow(ctx, store, apptest.SampleEpic, r.ID, apptest.SampleStatusInProgress, apptest.SampleStatusDone); err != nil {
		t.Fatal(err)
	}
	var it item.Item
	_ = rec.Read(ctx, store, func(rd port.Reader) error {
		it, _ = rd.Items().Get(apptest.SampleEpic)
		return nil
	})
	if it.Status != apptest.SampleStatusDone {
		t.Errorf("status = %q, want Done", it.Status)
	}
	if last := rec.summaries[len(rec.summaries)-1]; last != "「完了で外す」の変更を制約を無視して適用" {
		t.Errorf("summary = %q", last)
	}
}
