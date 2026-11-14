package repairing_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

var hanako = change.User("hanako")

// adminRule は、プロジェクト HH-P2「管理」の専用ルール「完了で閉じる」。
var adminRule = automation.Rule{ID: ownRule, Name: "完了で閉じる", Trigger: automation.TriggerPRMerged, Action: automation.ActionStatus, Value: string(apptest.SampleStatusDone), Enabled: true, Scope: project2}

// addAdminRule は、サンプルのワークスペースのログに、HH-P2 の専用ルールを書く。
func addAdminRule(e *env) {
	e.write(taro, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, automation.Drafts(automation.Rule{}, adminRule)...)
	})
}

// renameAdminRule は、hanako が HH-P2 の専用ルールの名前を変える。足した行の ID を返す。
func renameAdminRule(e *env) map[change.ID]bool {
	renamed := adminRule
	renamed.Name = "完了なら閉じる"
	return e.write(hanako, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, automation.Drafts(adminRule, renamed)...)
	})
}

// wantNoRestore は、ops に削除の取り消しがないことを確かめる。
func wantNoRestore(t *testing.T, ops []port.Operation) {
	t.Helper()
	for _, op := range ops {
		if strings.Contains(op.Summary, "の削除を取り消し") {
			t.Errorf("op %q restores the move", op.Summary)
		}
	}
}

// taro が HH-P2 を同じ Store のワークスペース「Mobile」へ移し(専用ルールの削除と転送記録を同じ op で書く)、hanako が
// 同時にその専用ルールの名前を変えても、移動の削除は削除と変更の重なりに数えない。HH-P2 は「Mobile」のまま。
func TestNoRestoreForMoveWithinStore(t *testing.T) {
	e := newEnv(t)
	addMobile(e)
	addAdminRule(e)
	moved := e.write(taro, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteProjects().Append(project2, change.NewSet("place", project.Place{Workspace: apptest.SampleWorkspace, Number: 2}, project.Place{Workspace: mobileWS, Number: 2})),
			tx.WriteWorkspaces().Append(mobileWS, numbering.Draft(numbering.Projects, 2)),
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, automation.DeleteDraft(ownRule)),
			tx.WriteTransfers().Append(transfer.Record{Entity: transfer.EntityProject, Target: project2, Old: "HH-P2", New: "MB-P2", ToStore: string(store)}),
		)
	})
	renamed := renameAdminRule(e)
	wantNoRestore(t, e.run(moved, renamed))
	if p := e.project(project2); p.Place.Workspace != mobileWS {
		t.Errorf("HH-P2 is in %s, want Mobile", p.Place.Workspace)
	}
}

// taro が HH-P2 を別の Store へ移し(フォルダを消し、専用ルールの削除と転送記録を同じ op で書く)、hanako が同時にその
// 専用ルールの名前を変えても、移動の削除は削除と変更の重なりに数えない。
func TestNoRestoreForMoveToOtherStore(t *testing.T) {
	e := newEnv(t)
	addAdminRule(e)
	moved := e.write(taro, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteProjects().Remove(project2),
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, automation.DeleteDraft(ownRule)),
			tx.WriteTransfers().Append(transfer.Record{Entity: transfer.EntityProject, Target: project2, Old: "HH-P2", New: "WEB-P1", ToStore: "S2", ToStoreName: "team-b"}),
		)
	})
	renamed := renameAdminRule(e)
	wantNoRestore(t, e.run(moved, renamed))
}

// 移動でない、専用ルールそのものの削除と名前の変更が重なったら、これまでどおり削除を取り消す。
func TestRestoreDedicatedRuleDeletedPlainly(t *testing.T) {
	e := newEnv(t)
	addAdminRule(e)
	deleted := e.write(taro, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, automation.DeleteDraft(ownRule))
	})
	renamed := renameAdminRule(e)
	wantOps(t, e.run(deleted, renamed), "「完了なら閉じる」の削除を取り消し")
}
