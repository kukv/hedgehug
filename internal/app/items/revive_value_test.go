package items_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// restores は、アイテムのログの restore の行数。
func restores(t *testing.T, s port.Stores, id item.ID) int {
	t.Helper()
	return len(slices.DeleteFunc(records(t, s, id), func(r change.Record) bool { return r.T != change.Restore }))
}

// 別のタブで削除された(root: true)Task の担当者を保存すると、変更を残して削除を取り消す。
func TestSetPropertyDeletedItemRevives(t *testing.T) {
	svc, rec := setupRec(t)
	markDeleted(t, rec, apptest.SampleTask, true)
	ctx := port.WithRevived(t.Context())
	got, err := svc.SetProperty(ctx, store, apptest.SampleTask, "assignee", raw(`"taro"`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Deleted != nil || got.Assignee != "taro" {
		t.Errorf("SetProperty = %+v", got)
	}
	if r := port.Revived(ctx); !slices.Equal(r, []string{"HH-2"}) {
		t.Errorf("Revived = %v", r)
	}
	if last(rec) != "HH-2 の担当者を taro に変更" {
		t.Errorf("summary = %q", last(rec))
	}
}

// 削除済みの Task のステータスを保存すると、自動化が動いても、restore は 1 行だけ。
func TestSetStatusDeletedItemWithAutomationRevivesOnce(t *testing.T) {
	svc, rec, _ := setupAuto(t, autoRule("01RULE1", automation.TriggerStatus, apptest.SampleStatusDone, automation.ActionAssignee, "taro"))
	markDeleted(t, rec, apptest.SampleTask, true)
	ctx := port.WithRevived(t.Context())
	got, err := svc.SetProperty(ctx, store, apptest.SampleTask, "status", raw(`"`+string(apptest.SampleStatusDone)+`"`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Deleted != nil || got.Status != apptest.SampleStatusDone || got.Assignee != "taro" {
		t.Errorf("SetProperty = %+v", got)
	}
	if n := restores(t, rec, apptest.SampleTask); n != 1 {
		t.Errorf("restore = %d 行, want 1", n)
	}
}

// 削除済みの Step のチェックを保存すると Step の削除を取り消し、削除されていない親の自動化が動く。
func TestCheckStepDeletedRevives(t *testing.T) {
	svc, rec, _ := setupAuto(t, autoRule("01RULE1", automation.TriggerStepsDone, "", automation.ActionStatus, string(apptest.SampleStatusDone)))
	step := must(t)(svc.AddStep(t.Context(), store, apptest.SampleTask, "設計"))
	markDeleted(t, rec, step.ID, true)
	ctx := port.WithRevived(t.Context())
	got, err := svc.CheckStep(ctx, store, step.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if got.Deleted != nil || got.Status != apptest.SampleStatusDone {
		t.Errorf("CheckStep = %+v", got)
	}
	if task := storedItem(t, rec, apptest.SampleTask); task.Status != apptest.SampleStatusDone {
		t.Errorf("親 = %+v, 自動化が動いていない", task)
	}
	if n := restores(t, rec, step.ID); n != 1 {
		t.Errorf("restore = %d 行, want 1", n)
	}
}

// 削除済みの Task に関連システムを足すと、削除を取り消す。
func TestToggleValueDeletedItemRevives(t *testing.T) {
	svc, rec := setupRec(t)
	markDeleted(t, rec, apptest.SampleTask, true)
	ctx := port.WithRevived(t.Context())
	got, err := svc.AddValue(ctx, store, apptest.SampleTask, "systems", apptest.SampleSystemWeb)
	if err != nil {
		t.Fatal(err)
	}
	if got.Deleted != nil || !slices.Equal(got.Systems, []change.ID{apptest.SampleSystemWeb}) {
		t.Errorf("AddValue = %+v", got)
	}
	if r := port.Revived(ctx); !slices.Equal(r, []string{"HH-2"}) {
		t.Errorf("Revived = %v", r)
	}
}

// 値が同じなら何も保存せず、削除済みのまま 404 になる(決定 S4)。
func TestSetPropertyDeletedItemSameValueIsNotFound(t *testing.T) {
	svc, rec := setupRec(t)
	markDeleted(t, rec, apptest.SampleTask, true)
	same := storedItem(t, rec, apptest.SampleTask).Status
	ctx := port.WithRevived(t.Context())
	if _, err := svc.SetProperty(ctx, store, apptest.SampleTask, "status", raw(`"`+string(same)+`"`)); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("SetProperty = %v, want port.ErrNotFound", err)
	}
	// 入っていない値を外すのも、何も変わらない。
	if _, err := svc.RemoveValue(ctx, store, apptest.SampleTask, "systems", apptest.SampleSystemWeb); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("RemoveValue = %v, want port.ErrNotFound", err)
	}
	if storedItem(t, rec, apptest.SampleTask).Deleted == nil || hasRestore(t, rec) {
		t.Error("何も変わらない保存で削除を取り消した")
	}
}

// 親の Epic と一緒に削除された(root: false)Task は、利用者が選んで削除していないので保存できない。
func TestSetPropertyItemDeletedWithParentIsNotFound(t *testing.T) {
	svc, rec := setupRec(t)
	if err := apptest.DeleteItems(t.Context(), rec, store, apptest.SampleEpic, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "assignee", raw(`"taro"`)); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("SetProperty = %v, want port.ErrNotFound", err)
	}
	if hasRestore(t, rec) || storedItem(t, rec, apptest.SampleTask).Deleted == nil {
		t.Error("root: false の削除を取り消した")
	}
}

// Task を削除したあと、親の Epic も別の保存で削除された。Task は祖先が削除済みなので保存できない(決定 S9)。
func TestSetPropertyUnderParentDeletedLaterIsNotFound(t *testing.T) {
	svc, rec := setupRec(t)
	markDeleted(t, rec, apptest.SampleTask, true)
	markDeleted(t, rec, apptest.SampleEpic, true)
	if _, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "assignee", raw(`"taro"`)); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("SetProperty = %v, want port.ErrNotFound", err)
	}
	if hasRestore(t, rec) || storedItem(t, rec, apptest.SampleTask).Deleted == nil {
		t.Error("祖先が削除済みなのに削除を取り消した")
	}
}

// 開始日を終了日より後にする保存は、削除済み(root: true)でも断られ、削除を取り消さない。
func TestSetPropertyDeletedItemRefusedDatesKeepsDeletion(t *testing.T) {
	svc, rec := setupRec(t)
	if _, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "end", raw(`"2026-11-10"`)); err != nil {
		t.Fatal(err)
	}
	markDeleted(t, rec, apptest.SampleTask, true)
	if _, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "start", raw(`"2026-11-20"`)); err == nil {
		t.Fatal("終了日より後の開始日が保存できた")
	}
	if hasRestore(t, rec) || storedItem(t, rec, apptest.SampleTask).Deleted == nil {
		t.Error("断られた保存で削除を取り消した")
	}
}
