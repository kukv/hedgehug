package repairing_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

const (
	webProject  project.ID = "01SAMPLEPR000000000000000A"
	ownRule     change.ID  = "01RULE00000000000000000002"
	orphanTail             = "」のルールを削除(プロジェクトがない)"
	orphanWebOp            = "プロジェクト「Web" + orphanTail
)

// addOrphanRules は、プロジェクト p の使わないステータス「Review」と、専用ルール「完了で閉じる」を、
// サンプルのワークスペースのログに書く(p がそのワークスペースにあるかは問わない)。
func (e *env) addOrphanRules(p project.ID) {
	rule := automation.Rule{ID: ownRule, Name: "完了で閉じる", Trigger: automation.TriggerPRMerged, Action: automation.ActionStatus, Value: string(apptest.SampleStatusDone), Enabled: true, Scope: p}
	e.write(taro, func(tx port.Tx) error {
		ds := append([]change.Draft{change.NewAdd(projectrule.HiddenField(p), review)}, automation.Drafts(automation.Rule{}, rule)...)
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, ds...)
	})
}

// addWebInMobile は、ワークスペース「Mobile」に、プロジェクト「Web」を足す。
func (e *env) addWebInMobile() {
	addMobile(e)
	web := project.Project{ID: webProject, Place: project.Place{Workspace: mobileWS, Number: 2}, Rank: "W", Title: "Web", Owner: "taro", Status: project.Todo}
	e.write(taro, func(tx port.Tx) error { return tx.WriteProjects().Append(web.ID, project.Create(web)) })
}

func (e *env) sampleRules() (projectrule.Set, []automation.Rule) {
	log := e.logOf(change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace})
	return projectrule.FromLog(log), automation.FromLog(log)
}

// 別のワークスペースへ移ったプロジェクトのルールは、元のワークスペースのログから外す。
func TestDropOrphanRulesMovedWithinStore(t *testing.T) {
	e := newEnv(t)
	e.addWebInMobile()
	e.addOrphanRules(webProject)
	wantOps(t, e.run(nil, nil), orphanWebOp)
	set, rules := e.sampleRules()
	if !set.Of(webProject).Empty() {
		t.Errorf("project rules = %+v", set.Of(webProject))
	}
	for _, r := range rules {
		if r.ID == ownRule {
			t.Errorf("own rule remains: %+v", r)
		}
	}
}

// プロジェクトを読めないときの {タイトル} は、転送記録の old、purged.jsonl の display、内部 ID の順。
func TestDropOrphanRulesTitleFallback(t *testing.T) {
	const gone project.ID = "01SAMPLEPR000000000000000B"
	tr := transfer.Record{Entity: transfer.EntityProject, Target: gone, ToStore: "S2"}
	for name, tc := range map[string]struct {
		prepare func(e *env)
		want    string
	}{
		"transfer old": {func(e *env) {
			old := tr
			old.Old = "HH-P2"
			last := tr
			last.Old = "HH-P3"
			e.write(taro, func(tx port.Tx) error {
				return errors.Join(tx.WriteTransfers().Append(old), tx.WriteTransfers().Append(last))
			})
			e.s.AddPurged(store, port.Purged{Entity: "project", Target: gone, Display: "HH-P4"})
		}, "プロジェクト「HH-P3" + orphanTail},
		"purged display": {func(e *env) {
			e.s.AddPurged(store, port.Purged{Entity: "project", Target: gone, Display: "HH-P4"})
		}, "プロジェクト「HH-P4" + orphanTail},
		"internal id": {func(*env) {}, "プロジェクト「" + string(gone) + orphanTail},
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			tc.prepare(e)
			e.addOrphanRules(gone)
			wantOps(t, e.run(nil, nil), tc.want)
		})
	}
}

// 削除済みのプロジェクトはワークスペースにあるので、そのルールは外さない。
func TestDropOrphanRulesKeepsDeleted(t *testing.T) {
	e := newEnv(t)
	e.addOrphanRules(project2)
	e.write(taro, func(tx port.Tx) error {
		return tx.WriteProjects().Append(project2, change.NewDeleteEntity(true))
	})
	wantOps(t, e.run(nil, nil))
	if set, _ := e.sampleRules(); set.Of(project2).Empty() {
		t.Error("rules of the deleted project were removed")
	}
}

// 1 回の Run で (8) と (9) が両方要るときは、(8) の保存のあとに (9) の保存を書く。
func TestDropOrphanRulesAfterPlan(t *testing.T) {
	e := newEnv(t)
	e.addWebInMobile()
	e.addOrphanRules(webProject)
	e.appendItem(taro, apptest.SampleTask, change.NewSet(item.SprintField, nil, sprint9))
	wantOps(t, e.run(nil, nil), "HH-2 をバックログに戻す(スプリントが完全に削除)", orphanWebOp)
}
