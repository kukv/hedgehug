package automations_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// 親と一緒に削除された(root: false)アイテムには動かない。通知の「制約を無視して {ステータス名} にする」は、
// 利用者が選んで削除していないアイテムなら、ないものと同じ(06「通知」のボタンを出さない条件。A4b-5)。
func TestApplyIgnoringWorkflowRefusesADeletedItem(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	r, err := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	if err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteItems(ctx, rec.Stores, store, apptest.SampleEpic, apptest.SampleTask); err != nil {
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

// 削除済み(root: true)のアイテムへの「制約を無視して適用」は、変更を残して削除を取り消す(06「他の書き手との重なり」)。
func TestApplyIgnoringWorkflowRevivesADeletedItem(t *testing.T) {
	svc, rec := setup(t)
	ctx := port.WithRevived(t.Context())
	r, err := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	if err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteItems(ctx, rec.Stores, store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyIgnoringWorkflow(ctx, store, apptest.SampleTask, r.ID, apptest.SampleStatusDone); err != nil {
		t.Fatal(err)
	}
	err = rec.Read(ctx, store, func(rd port.Reader) error {
		it, err := rd.Items().Get(apptest.SampleTask)
		if err != nil || it.Deleted != nil || it.Status != apptest.SampleStatusDone {
			t.Errorf("item = %+v, %v", it, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := port.Revived(ctx); !slices.Equal(got, []string{"HH-2"}) {
		t.Errorf("Revived = %v", got)
	}
}

// 同じステータスなら何も保存せず、削除済みのまま 404 になる(決定 S4)。
func TestApplyIgnoringWorkflowSameStatusOnADeletedItemIsNotFound(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	r, err := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	if err != nil {
		t.Fatal(err)
	}
	var status change.ID
	_ = rec.Read(ctx, store, func(rd port.Reader) error {
		it, _ := rd.Items().Get(apptest.SampleTask)
		status = it.Status
		return nil
	})
	if err := apptest.DeleteItems(ctx, rec.Stores, store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyIgnoringWorkflow(ctx, store, apptest.SampleTask, r.ID, status); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("err = %v, want port.ErrNotFound", err)
	}
}
