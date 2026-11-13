package items_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
)

// setStatusRow は、行を直接書いてステータスを変える(遷移も自動化も通さない)。
func setStatusRow(t *testing.T, s port.Stores, id item.ID, to change.ID) {
	t.Helper()
	from := storedItem(t, s, id).Status
	op := port.Operation{Actor: change.User("taro"), Summary: "ステータスを変更"}
	if err := s.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteItems().Append(id, change.NewSet("status", from, to))
	}); err != nil {
		t.Fatal(err)
	}
}

// movePendingSetup は、決済プロジェクトが Pending を外し、Epic HH-1 の下に Pending の Task HH-2 と
// Pending の削除済みの Task HH-3 がある状態。
func movePendingSetup(t *testing.T, rules ...automation.Rule) (items.Service, *recorder, item.Item) {
	t.Helper()
	svc, rec, _ := setupAuto(t, rules...)
	addSecondProject(t, rec)
	gone := mustCreate(t, svc, apptest.SampleProject, ptr(apptest.SampleEpic), item.Task, "消した子") // HH-3
	setStatusRow(t, rec, apptest.SampleTask, apptest.SampleStatusPending)
	setStatusRow(t, rec, gone.ID, apptest.SampleStatusPending)
	markDeleted(t, rec, gone.ID, false)
	writeWorkspace(t, rec, change.NewAdd(projectrule.HiddenField(otherProject), apptest.SampleStatusPending))
	return svc, rec, gone
}

func TestMovePreviewCountsDeletedChildren(t *testing.T) {
	svc, _, _ := movePendingSetup(t)
	got, err := svc.MovePreview(t.Context(), store, apptest.SampleEpic, items.MoveInput{Project: pptr(otherProject)})
	if err != nil {
		t.Fatal(err)
	}
	// 下の件数は削除済みを数えず(HH-2 だけ)、ステータスの件数は削除済みを含む(HH-2 と HH-3)。
	want := items.MovePreview{Below: 1, Statuses: []items.StatusCount{{Status: apptest.SampleStatusPending, Count: 2, Default: apptest.SampleStatusTodo}}}
	if got.Below != want.Below || !slices.Equal(got.Statuses, want.Statuses) {
		t.Errorf("preview = %+v, want %+v", got, want)
	}
}

func TestMovePreviewIsEmptyWithinAProject(t *testing.T) {
	svc, _, _ := movePendingSetup(t)
	epic2 := mustCreate(t, svc, apptest.SampleProject, nil, item.Epic, "決済")
	got, err := svc.MovePreview(t.Context(), store, apptest.SampleTask, items.MoveInput{Parent: &epic2.ID})
	if err != nil || got.Below != 0 || len(got.Statuses) != 0 {
		t.Errorf("preview = %+v, %v", got, err)
	}
}

func TestMoveReplacesUnusedStatuses(t *testing.T) {
	// 置き換えは自動化を動かさない: InProgress になったら担当者を taro にするルールがあっても、担当者は変わらない。
	rule := autoRule("01RULE1", automation.TriggerStatus, apptest.SampleStatusInProgress, automation.ActionAssignee, "taro")
	svc, rec, gone := movePendingSetup(t, rule)
	in := items.MoveInput{Project: pptr(otherProject), Statuses: map[change.ID]change.ID{apptest.SampleStatusPending: apptest.SampleStatusInProgress}}
	if err := svc.Move(t.Context(), store, apptest.SampleEpic, in); err != nil {
		t.Fatal(err)
	}
	for _, id := range []item.ID{apptest.SampleTask, gone.ID} {
		if it := storedItem(t, rec, id); it.Status != apptest.SampleStatusInProgress || it.Assignee != "" {
			t.Errorf("%s: status = %s, assignee = %q", id, it.Status, it.Assignee)
		}
	}
	if got := storedItem(t, rec, apptest.SampleEpic); got.Status != apptest.SampleStatusTodo {
		t.Errorf("使うステータスは変えない: %s", got.Status)
	}
	// 移動と同じ操作で書かれ、履歴に残る。
	if ops := lastOps(t, rec, apptest.SampleTask, 2); ops[0] != ops[1] {
		t.Errorf("移動と置き換えは同じ操作: %v", ops)
	}
	log := records(t, rec, apptest.SampleTask)
	if r := log[len(log)-1]; r.T != change.Set || r.F != "status" || string(r.From) != `"`+string(apptest.SampleStatusPending)+`"` || string(r.To) != `"`+string(apptest.SampleStatusInProgress)+`"` {
		t.Errorf("履歴に残る行 = %+v", r)
	}
}

func TestMoveReplacesWithTheSameCategoryDefaultWhenNotChosen(t *testing.T) {
	svc, rec, gone := movePendingSetup(t)
	if err := svc.Move(t.Context(), store, apptest.SampleEpic, items.MoveInput{Project: pptr(otherProject)}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []item.ID{apptest.SampleTask, gone.ID} {
		if it := storedItem(t, rec, id); it.Status != apptest.SampleStatusTodo {
			t.Errorf("%s: status = %s, want Todo", id, it.Status)
		}
	}
}

func TestMoveRefusesUnusedReplacement(t *testing.T) {
	svc, rec, _ := movePendingSetup(t)
	before := logLen(t, rec, apptest.SampleTask)
	in := items.MoveInput{Project: pptr(otherProject), Statuses: map[change.ID]change.ID{apptest.SampleStatusPending: apptest.SampleStatusPending}}
	if err := svc.Move(t.Context(), store, apptest.SampleEpic, in); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if it := storedItem(t, rec, apptest.SampleTask); it.Place.Project != apptest.SampleProject || logLen(t, rec, apptest.SampleTask) != before {
		t.Errorf("断ったら何も保存しない: %+v", it.Place)
	}
}

func TestMoveWithinProjectKeepsStatus(t *testing.T) {
	svc, rec, _ := movePendingSetup(t)
	epic2 := mustCreate(t, svc, apptest.SampleProject, nil, item.Epic, "決済")
	in := items.MoveInput{Parent: &epic2.ID, Statuses: map[change.ID]change.ID{apptest.SampleStatusPending: apptest.SampleStatusDone}}
	if err := svc.Move(t.Context(), store, apptest.SampleTask, in); err != nil {
		t.Fatal(err)
	}
	if it := storedItem(t, rec, apptest.SampleTask); it.Status != apptest.SampleStatusPending {
		t.Errorf("同じプロジェクトの中ではステータスを変えない: %s", it.Status)
	}
}
