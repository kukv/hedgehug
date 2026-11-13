package items_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
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

// 別のワークスペースのブロッカーが完了・中止かは、ブロッカーのワークスペースの設定で決める(A4a-13)。
// ワークスペース「Mobile」では、Web の「未着手」の ID が完了の分類、Web の「完了」の ID が未着手の分類になるようにする。
func TestBlockedJudgesBlockerByItsOwnWorkspace(t *testing.T) {
	svc := setup(t)
	const (
		wsMobile workspace.ID = "01TESTWS00000000000000002"
		prMobile project.ID   = "01TESTPR00000000000000002"
		itMobile item.ID      = "01TESTIT00000000000000005"
	)
	n := 0
	valueIDs := func() change.ID {
		n++
		switch n {
		case 1: // 未着手
			return apptest.SampleStatusDone
		case 3: // 完了
			return apptest.SampleStatusTodo
		}
		return change.ID(fmt.Sprintf("01TESTVAL%016d", n))
	}
	d, err := workspace.CreateWithSettings(workspace.Workspace{ID: wsMobile, Name: "Mobile", Prefix: "MB"}, workspace.DefaultSettings(valueIDs))
	if err != nil {
		t.Fatal(err)
	}
	pr := project.Project{ID: prMobile, Place: project.Place{Workspace: wsMobile, Number: 1}, Rank: "V", Title: "アプリ", Status: project.Todo}
	blocker := item.Item{
		ID: itMobile, Kind: item.Epic, Rank: "V", Title: "iOS ログイン", Status: apptest.SampleStatusTodo,
		Place: item.Place{Workspace: wsMobile, Project: prMobile, Number: 1},
	}
	op := port.Operation{Actor: change.User("taro"), Summary: "テスト"}
	err = svc.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(wsMobile, d),
			tx.WriteProjects().Append(prMobile, project.Create(pr)),
			tx.WriteItems().Append(itMobile, item.Create(blocker),
				change.NewAdd("relations", item.Relation{Type: "blocks", To: apptest.SampleTask})),
		)
	})
	if err != nil {
		t.Fatal(err)
	}
	blocked := func() bool {
		got, err := svc.Get(t.Context(), store, apptest.SampleTask)
		if err != nil {
			t.Fatal(err)
		}
		return got.Blocked
	}
	if blocked() {
		t.Error("Mobile で完了の分類のブロッカーではブロック中にならない")
	}
	err = svc.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteItems().Append(itMobile, change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusDone))
	})
	if err != nil {
		t.Fatal(err)
	}
	if !blocked() {
		t.Error("Mobile で未着手の分類のブロッカーならブロック中")
	}
}
