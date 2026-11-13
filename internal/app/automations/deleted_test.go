package automations_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// 削除済みのアイテムには動かない。通知の「制約を無視して {ステータス名} にする」は、アイテムが削除済みなら
// ないものと同じ(06「通知」のボタンを出さない条件。A4b-5)。
func TestApplyIgnoringWorkflowRefusesADeletedItem(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	r, err := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	if err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteItems(ctx, rec.Stores, store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyIgnoringWorkflow(ctx, store, apptest.SampleTask, r.ID, apptest.SampleStatusDone); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("err = %v, want port.ErrNotFound", err)
	}
	err = rec.Read(ctx, store, func(rd port.Reader) error {
		log, err := rd.Changes().Of(change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
		if err != nil {
			return err
		}
		if last := log[len(log)-1]; last.T != change.Delete {
			t.Errorf("a refused apply wrote %+v", last)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// 最近の実行は、動いたときの記録なので、そのアイテムが削除されても残す(A4b-5。削除済みを除かない)。
func TestRecentRunsKeepADeletedItemsRun(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	r, err := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	if err != nil {
		t.Fatal(err)
	}
	writeRuleRow(t, rec.Stores, r.ID, nil, "taro")
	if err := apptest.DeleteItems(ctx, rec.Stores, store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	v, err := svc.List(ctx, store, apptest.SampleWorkspace)
	if err != nil || len(v.Recent) != 1 || v.Recent[0].Display != "HH-2" {
		t.Errorf("Recent = %+v, %v", v.Recent, err)
	}
}
