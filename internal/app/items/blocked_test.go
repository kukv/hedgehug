package items_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

func writeLog(t *testing.T, s port.Stores, d change.Draft) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "テスト"}
	err := s.Write(t.Context(), store, op, func(tx port.Tx) error { return tx.WriteItems().Append(apptest.SampleEpic, d) })
	if err != nil {
		t.Fatal(err)
	}
}

// ブロックしている相手が完了か中止でなければ、ブロック中(A4a-13)。Get・List・FindByNumber のどれでも同じ。
func TestBlockedFollowsTheBlockersStatus(t *testing.T) {
	svc := setup(t)
	ctx := t.Context()
	blocked := func() [3]bool {
		got, err := svc.Get(ctx, store, apptest.SampleTask)
		if err != nil {
			t.Fatal(err)
		}
		list, err := svc.List(ctx, store, apptest.SampleProject)
		if err != nil {
			t.Fatal(err)
		}
		found, err := svc.FindByNumber(ctx, store, apptest.SampleWorkspace, 2)
		if err != nil {
			t.Fatal(err)
		}
		return [3]bool{got.Blocked, list[1].Blocked, found.Blocked}
	}
	if blocked() != [3]bool{} {
		t.Fatalf("関連がなければブロック中でない: %v", blocked())
	}
	// HH-1(未着手)が HH-2 をブロックする。
	writeLog(t, svc.Stores, change.NewAdd("relations", item.Relation{Type: "blocks", To: apptest.SampleTask}))
	if blocked() != [3]bool{true, true, true} {
		t.Errorf("未着手のブロッカーがいればブロック中: %v", blocked())
	}
	if list, _ := svc.List(ctx, store, apptest.SampleProject); list[0].Blocked {
		t.Error("ブロックしている側(HH-1)はブロック中でない")
	}
	// ほかの種類の関連は関係しない。
	writeLog(t, svc.Stores, change.NewRemove("relations", item.Relation{Type: "blocks", To: apptest.SampleTask}))
	writeLog(t, svc.Stores, change.NewAdd("relations", item.Relation{Type: "causes", To: apptest.SampleTask}))
	if blocked() != [3]bool{} {
		t.Errorf("causes ではブロック中にならない: %v", blocked())
	}
	// ブロッカーが完了になったら外れる。
	writeLog(t, svc.Stores, change.NewAdd("relations", item.Relation{Type: "blocks", To: apptest.SampleTask}))
	writeLog(t, svc.Stores, change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusDone))
	if blocked() != [3]bool{} {
		t.Errorf("完了のブロッカーでは外れる: %v", blocked())
	}
	writeLog(t, svc.Stores, change.NewSet("status", apptest.SampleStatusDone, apptest.SampleStatusCancel))
	if blocked() != [3]bool{} {
		t.Errorf("中止のブロッカーでも外れる: %v", blocked())
	}
	writeLog(t, svc.Stores, change.NewSet("status", apptest.SampleStatusCancel, apptest.SampleStatusInProgress))
	if blocked() != [3]bool{true, true, true} {
		t.Errorf("進行中なら再びブロック中: %v", blocked())
	}
}
