package items_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// 削除済みのアイテムは、一覧にも、Get・FindByNumber にも出ず、変更もできない(A4b-4)。
func TestDeletedItemIsHiddenAndCannotBeChanged(t *testing.T) {
	svc := setup(t)
	ctx := t.Context()
	if err := apptest.DeleteItems(ctx, svc.Stores, store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	list, err := svc.List(ctx, store, apptest.SampleProject)
	if err != nil || len(list) != 1 || list[0].ID != apptest.SampleEpic {
		t.Errorf("List = %+v, %v", list, err)
	}
	_, getErr := svc.Get(ctx, store, apptest.SampleTask)
	_, findErr := svc.FindByNumber(ctx, store, apptest.SampleWorkspace, 2)
	_, renameErr := svc.Rename(ctx, store, apptest.SampleTask, "新しい")
	_, setErr := svc.SetProperty(ctx, store, apptest.SampleTask, "assignee", raw(`"taro"`))
	_, stepErr := svc.AddStep(ctx, store, apptest.SampleTask, "下書き")
	for name, err := range map[string]error{"Get": getErr, "FindByNumber": findErr, "Rename": renameErr, "SetProperty": setErr, "AddStep": stepErr} {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s of a deleted item: %v, want port.ErrNotFound", name, err)
		}
	}
	// 削除していないアイテムは今までどおり読める。
	if _, err := svc.Get(ctx, store, apptest.SampleEpic); err != nil {
		t.Errorf("Get(HH-1) = %v", err)
	}
}

// 削除済みのプロジェクトのアイテムの一覧は、プロジェクトがないときと同じ(A4b-4)。
func TestListOfADeletedProjectIsNotFound(t *testing.T) {
	svc := setup(t)
	if err := apptest.DeleteProject(t.Context(), svc.Stores, store, apptest.SampleProject); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.List(t.Context(), store, apptest.SampleProject); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("List = %v", err)
	}
}

// 兄弟の末尾の決め方は削除済みの並び順の値を見ない。番号は削除済みも数えるので、再利用しない(A4b-5)。
func TestCreateIgnoresDeletedSiblingsButNeverReusesTheirNumbers(t *testing.T) {
	svc := setup(t)
	ctx := t.Context()
	in := items.CreateInput{Store: store, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Kind: item.Task, Title: "画面"}
	last, err := svc.Create(ctx, in) // HH-3、並びは W(サンプルの HH-2 の V のあと)
	if err != nil || last.Rank != "W" || last.Place.Number != 3 {
		t.Fatalf("HH-3 = %+v, %v", last, err)
	}
	if err := apptest.DeleteItems(ctx, svc.Stores, store, last.ID); err != nil {
		t.Fatal(err)
	}
	again, err := svc.Create(ctx, in)
	if err != nil || again.Rank != "W" || again.Place.Number != 4 {
		t.Errorf("after deleting HH-3: %+v, %v; want rank W (the deleted one is not compared) and number 4", again, err)
	}
}

// 削除済みの親・プロジェクトの下には作れない(A4b-4)。
func TestCreateUnderADeletedParentOrProjectIsNotFound(t *testing.T) {
	svc := setup(t)
	ctx := t.Context()
	if err := apptest.DeleteItems(ctx, svc.Stores, store, apptest.SampleEpic, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Create(ctx, items.CreateInput{Store: store, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Kind: item.Task, Title: "画面"})
	if !errors.Is(err, port.ErrNotFound) {
		t.Errorf("under a deleted parent: %v", err)
	}
	if err := apptest.DeleteProject(ctx, svc.Stores, store, apptest.SampleProject); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Create(ctx, items.CreateInput{Store: store, Project: apptest.SampleProject, Kind: item.Epic, Title: "決済"})
	if !errors.Is(err, port.ErrNotFound) {
		t.Errorf("in a deleted project: %v", err)
	}
}

// ブロックしている相手が削除済みなら、ブロック中に数えない(A4b-5。関連そのものは消さない)。
func TestADeletedBlockerDoesNotBlock(t *testing.T) {
	svc := setup(t)
	ctx := t.Context()
	// HH-1(未着手)が HH-2 をブロックする。
	writeLog(t, svc.Stores, change.NewAdd("relations", item.Relation{Type: "blocks", To: apptest.SampleTask}))
	if got := must(t)(svc.Get(ctx, store, apptest.SampleTask)); !got.Blocked {
		t.Fatal("HH-2 は HH-1 にブロックされている")
	}
	if err := apptest.DeleteItems(ctx, svc.Stores, store, apptest.SampleEpic); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Get(ctx, store, apptest.SampleTask)
	if err != nil || got.Blocked {
		t.Errorf("Get = %+v, %v; a deleted blocker must not block", got, err)
	}
	if list, err := svc.List(ctx, store, apptest.SampleProject); err != nil || len(list) != 1 || list[0].Blocked {
		t.Errorf("List = %+v, %v", list, err)
	}
}

// すべての Step が完了したかは、削除済みの Step を数えない(A4b-5)。
func TestStepsDoneIgnoresDeletedSteps(t *testing.T) {
	svc, _, inbox := setupAuto(t, autoRule("01RULE1", automation.TriggerStepsDone, "", automation.ActionStatus, string(apptest.SampleStatusDone)))
	ctx := t.Context()
	a := must(t)(svc.AddStep(ctx, store, apptest.SampleTask, "設計"))
	b := must(t)(svc.AddStep(ctx, store, apptest.SampleTask, "実装"))
	if err := apptest.DeleteItems(ctx, svc.Stores, store, b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CheckStep(ctx, store, a.ID, true); err != nil {
		t.Fatal(err)
	}
	task := must(t)(svc.Get(ctx, store, apptest.SampleTask))
	if task.Status != apptest.SampleStatusDone || len(inbox.Notices()) != 1 {
		t.Errorf("task = %+v, notices = %+v; the deleted step must not hold the rule back", task, inbox.Notices())
	}
}

// 親が削除済みなら、Step の変更はできるが、親のルールは動かない(03「実行の規則」。A4b-5)。
func TestStepsDoneDoesNotRunOnADeletedParent(t *testing.T) {
	svc, rec, inbox := setupAuto(t, autoRule("01RULE1", automation.TriggerStepsDone, "", automation.ActionStatus, string(apptest.SampleStatusDone)))
	ctx := t.Context()
	a := must(t)(svc.AddStep(ctx, store, apptest.SampleTask, "設計"))
	if err := apptest.DeleteItems(ctx, svc.Stores, store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CheckStep(ctx, store, a.ID, true); err != nil {
		t.Fatalf("CheckStep under a deleted parent = %v", err)
	}
	if n := len(records(t, rec.Stores, apptest.SampleTask)); n != 2 || len(inbox.Notices()) != 0 {
		t.Errorf("the deleted parent got %d rows, notices = %+v; want 2 rows (create and delete) and no notices", n, inbox.Notices())
	}
}

// 並べ替えは、削除済みの兄弟をいないものとして数える(A4b-5)。
func TestReorderSkipsDeletedSiblings(t *testing.T) {
	svc := setup(t)
	ctx := t.Context()
	in := items.CreateInput{Store: store, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Kind: item.Task, Title: "画面"}
	mid := must(t)(svc.Create(ctx, in)) // HH-3(W)
	last := must(t)(svc.Create(ctx, in))
	if err := apptest.DeleteItems(ctx, svc.Stores, store, mid.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Reorder(ctx, store, last.ID, apptest.SampleTask, false); err != nil {
		t.Fatal(err)
	}
	moved := must(t)(svc.Get(ctx, store, last.ID))
	first := must(t)(svc.Get(ctx, store, apptest.SampleTask))
	if moved.Rank >= first.Rank {
		t.Errorf("moved = %q, HH-2 = %q; the last one must come before HH-2", moved.Rank, first.Rank)
	}
	if n := len(records(t, svc.Stores, mid.ID)); n != 2 {
		t.Errorf("the deleted sibling got %d rows, want 2 (create and delete; the rank is untouched)", n)
	}
	if err := svc.Reorder(ctx, store, last.ID, mid.ID, false); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("a deleted anchor: %v", err)
	}
}

// must は、準備の呼び出しの値を返す。エラーならテストを止める。
func must(t *testing.T) func(item.Item, error) item.Item {
	t.Helper()
	return func(it item.Item, err error) item.Item {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return it
	}
}

// 削除済みのワークスペースの次の番号は、ワークスペースがないときと同じ(B5b-4)。
func TestNextNumberOfDeletedWorkspace(t *testing.T) {
	svc := setup(t)
	if err := apptest.DeleteWorkspace(t.Context(), svc.Stores, store, apptest.SampleWorkspace); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.NextNumber(t.Context(), store, apptest.SampleWorkspace); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("NextNumber = %v, want port.ErrNotFound", err)
	}
}

// 削除したワークスペースと同じ名前・接頭辞で作り直したワークスペースの採番は、新しいワークスペースのログで数えるので 1 から始まる。
func TestNextNumberOfRecreatedWorkspace(t *testing.T) {
	svc := setup(t)
	ctx := t.Context()
	if err := apptest.DeleteWorkspace(ctx, svc.Stores, store, apptest.SampleWorkspace); err != nil {
		t.Fatal(err)
	}
	again := workspace.Workspace{ID: svc.IDs.New(), Name: "Web", Prefix: "HH"}
	op := port.Operation{Actor: change.User("taro"), Summary: "ワークスペースを作成"}
	err := svc.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		d, err := workspace.CreateWithSettings(again, workspace.DefaultSettings(svc.IDs.New))
		if err != nil {
			return err
		}
		return tx.WriteWorkspaces().Append(again.ID, d)
	})
	if err != nil {
		t.Fatal(err)
	}
	if n, err := svc.NextNumber(ctx, store, again.ID); err != nil || n != 1 {
		t.Errorf("NextNumber = %d, %v, want 1", n, err)
	}
}
