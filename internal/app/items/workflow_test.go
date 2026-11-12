package items_test

import (
	"encoding/json"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// restrict は、サンプルのワークスペースで、from から to にだけ変えられるようにする(B3 の行の形で書く)。
func restrict(t *testing.T, rec *recorder, from change.ID, to ...change.ID) {
	t.Helper()
	ds := make([]change.Draft, 0, len(to))
	for _, id := range to {
		ds = append(ds, change.NewAdd(workspace.WorkflowField(from), id))
	}
	err := rec.Stores.Write(t.Context(), store, port.Operation{Actor: change.User("hanako"), Summary: "ワークフローの遷移の制約を変更"}, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, ds...)
	})
	if err != nil {
		t.Fatal(err)
	}
}

const refusedTodoToDone = "ワークフローの制約: Todo から Done には変更できません(InProgress のみ)"

func TestSetPropertyRefusesAStatusTheWorkflowForbids(t *testing.T) {
	svc, rec := setupRec(t)
	restrict(t, rec, apptest.SampleStatusTodo, apptest.SampleStatusInProgress)
	before := logLen(t, rec, apptest.SampleTask)
	_, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "status", quoted(apptest.SampleStatusDone))
	if message(err) != refusedTodoToDone || logLen(t, rec, apptest.SampleTask) != before {
		t.Errorf("SetProperty = %v; log %d → %d", err, before, logLen(t, rec, apptest.SampleTask))
	}
	if _, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "status", quoted(apptest.SampleStatusInProgress)); err != nil {
		t.Errorf("an allowed status must be saved: %v", err)
	}
}

func TestCheckStepRefusesAStatusTheWorkflowForbids(t *testing.T) {
	svc, rec := setupRec(t)
	step, err := svc.AddStep(t.Context(), store, apptest.SampleTask, "確認")
	if err != nil {
		t.Fatal(err)
	}
	restrict(t, rec, apptest.SampleStatusTodo, apptest.SampleStatusInProgress)
	if _, err := svc.CheckStep(t.Context(), store, step.ID, true); message(err) != refusedTodoToDone {
		t.Errorf("CheckStep = %v", err)
	}
}

// 作成と Step の追加には遷移の制約を適用しない(A3b-3)。初期ステータス以外でも作れる。
func TestCreateAndAddStepIgnoreTheWorkflow(t *testing.T) {
	svc, rec := setupRec(t)
	restrict(t, rec, apptest.SampleStatusTodo, apptest.SampleStatusInProgress)
	_, err := svc.Create(t.Context(), items.CreateInput{
		Store: store, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Kind: "Task", Title: "完了で作る",
		Values: map[change.ID]json.RawMessage{"status": quoted(apptest.SampleStatusDone)},
	})
	if err != nil {
		t.Errorf("Create with Done = %v", err)
	}
	if _, err := svc.AddStep(t.Context(), store, apptest.SampleTask, "確認"); err != nil {
		t.Errorf("AddStep = %v", err)
	}
}
