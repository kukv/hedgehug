package transfers_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// undoFails は、2 回目の Deliver(移動先の保存の取り消し)を、何も保存せずに失敗させる Delivery。reason があれば
// 理由(送れなかった)を、なければ err を返す。1 回目は inner に渡す。
type undoFails struct {
	inner  transfers.Delivery
	reason syncstatus.Reason
	err    error
	calls  *int
}

var errUndo = errors.New("undo failed")

func (d undoFails) Deliver(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) (syncstatus.Reason, error) {
	*d.calls++
	if *d.calls == 1 {
		return d.inner.Deliver(ctx, id, op, fn)
	}
	return d.reason, d.err
}

// assertBothKept は、移動が UndoError で失敗し、移したアイテムが移動先と移動元の両方に残っていることを確かめる。
func assertBothKept(t *testing.T, f fixture, err error) {
	t.Helper()
	var ue *transfers.UndoError
	if !errors.As(err, &ue) {
		t.Fatalf("err = %v, want *UndoError", err)
	}
	if ue.Old != "HH-2" || ue.Store != "team" || ue.NewDisplay != "TM-2" {
		t.Errorf("UndoError = %+v", ue)
	}
	const want = "HH-2 を team の TM-2 に移しましたが、移動元から取り除けず、取り消しもできませんでした。どちらかを削除してください"
	if ue.Error() != want {
		t.Errorf("文 = %q", ue.Error())
	}
	if se := (*transfers.SendError)(nil); errors.As(err, &se) {
		t.Errorf("送信の失敗としても読める: %v", se)
	}
	if !exists(t, f.rec, store2, apptest.SampleTask) || !exists(t, f.rec, store1, apptest.SampleTask) {
		t.Error("移動先と移動元の両方に残る")
	}
}

// 移動元の保存が失敗し、取り消しも送れなかった(理由)・失敗した(エラー)ときは、UndoError を返す。
func TestMoveAcrossStoresReportsAnUndoThatFailed(t *testing.T) {
	for _, d := range []undoFails{{reason: syncstatus.WriteDenied}, {err: errUndo}} {
		f := crossFixture(t)
		f.svc.Stores = failWrite{Stores: f.rec, store: store1}
		d.inner, d.calls = f.deliver, new(int)
		f.svc.Delivery = d
		_, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toTeam())
		assertBothKept(t, f, err)
		if !errors.Is(err, errBoom) {
			t.Errorf("移動元の保存の失敗を包まない: %v", err)
		}
	}
}

// 移動先の保存が送れず捨てられもしなかったうえに、取り消しも失敗したときも、UndoError を返す。
func TestMoveAcrossStoresReportsAnUndoThatFailedAfterASaveThatWasNotDiscarded(t *testing.T) {
	f := crossFixture(t)
	f.svc.Delivery = notDiscarded{inner: undoFails{inner: f.deliver, err: errUndo, calls: new(int)}, done: new(bool)}
	_, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toTeam())
	assertBothKept(t, f, err)
}

// 移動元の保存の前に、移動先で移した Epic の下に子が作られていても、戻しはそれも消す(移動先に、親のないアイテムを残さない)。
func TestMoveAcrossStoresRollbackRemovesChildrenAddedAtTheDestination(t *testing.T) {
	const added item.ID = "01TESTIT000000000000000013"
	f := newFixture(t)
	f.svc.Stores = failWrite{Stores: f.rec, store: store1, before: func() {
		addItem(t, f.rec, store2, item.Item{ID: added, Place: item.Place{Workspace: teamWS, Project: teamProject, Parent: ptr(apptest.SampleEpic), Number: 9}, Kind: item.Task, Rank: "X", Title: "移動先で"})
	}}
	in := transfers.MoveInput{ToStore: store2, ToWorkspace: teamWS, ToProject: teamProject}
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleEpic, in); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
	if exists(t, f.rec, store2, added) {
		t.Error("the child added at the destination remains")
	}
	if exists(t, f.rec, store2, apptest.SampleEpic) || !exists(t, f.rec, store1, apptest.SampleEpic) {
		t.Error("the Epic was not rolled back")
	}
}
