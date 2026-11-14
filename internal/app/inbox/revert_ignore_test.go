package inbox_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ワークフローが Done → Todo を禁じていても、戻す保存は Todo を書く(C4c2-7)。
func TestRevertIgnoresWorkflow(t *testing.T) {
	f := withRevert(t)
	f.writeWorkspace(t, change.NewAdd(workspace.WorkflowField(apptest.SampleStatusDone), apptest.SampleStatusInProgress))
	f.setTask(t, change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusDone))
	id := f.notice(t, overwritten("status", apptest.SampleStatusTodo, apptest.SampleStatusDone))
	if err := f.svc.Revert(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	want := `set status "` + string(apptest.SampleStatusDone) + `" "` + string(apptest.SampleStatusTodo) + `"`
	if got := lastOp(f.logOf(t, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})); len(got) != 1 || got[0] != want {
		t.Errorf("last op = %q, want %q", got, want)
	}
}

// 「Todo になったら Done にする」ルールがあっても動かない。保存は戻しの 1 回だけ(C4c2-7)。
func TestRevertDoesNotRunAutomation(t *testing.T) {
	f := withRevert(t)
	rule := automation.Rule{
		ID: "01SAMPLERULE00000000000001", Name: "Todo なら Done", Trigger: automation.TriggerStatus,
		TriggerStatus: apptest.SampleStatusTodo, Action: automation.ActionStatus, Value: string(apptest.SampleStatusDone), Enabled: true,
	}
	f.writeWorkspace(t, automation.Drafts(automation.Rule{}, rule)...)
	f.setTask(t, change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusDone))
	id := f.notice(t, overwritten("status", apptest.SampleStatusTodo, apptest.SampleStatusDone))
	ref := change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}
	before := len(f.logOf(t, ref))
	if err := f.svc.Revert(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	log := f.logOf(t, ref)
	var status change.ID
	_ = f.stores.Read(t.Context(), s1, func(r port.Reader) error {
		it, _ := r.Items().Get(apptest.SampleTask)
		status = it.Status
		return nil
	})
	if len(log) != before+1 || status != apptest.SampleStatusTodo {
		t.Errorf("rows = %d (before %d), status = %s, want one new row and Todo", len(log), before, status)
	}
}
