//go:build !e2e

package main

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// moveWebToStoreAs は、login の操作として、「Web」を別の Store「team-b」の WEB-P2 へ移す移動元の保存を書く。
func moveWebToStoreAs(t *testing.T, m machine, login string) {
	t.Helper()
	mainLog := m.wsLog(t, mainWS)
	from, _ := workspace.SettingsFromLog(mainLog)
	own := slices.DeleteFunc(automation.FromLog(mainLog), func(r automation.Rule) bool { return r.Scope != webPR })
	carried := transfer.CarryRules(transfer.CarryInput{
		Project: webPR, From: from, To: from, Rules: projectrule.FromLog(mainLog).Of(webPR), Own: own,
		NewID: func() change.ID { return webCarried },
	})
	m.writeAs(t, login, "プロジェクト「Web」を team-b へ移動", func(tx port.Tx) error {
		return errors.Join(
			tx.WriteProjects().Remove(webPR),
			tx.WriteWorkspaces().Append(mainWS, carried.Source...),
			tx.WriteTransfers().Append(transfer.Record{Entity: transfer.EntityProject, Target: webPR, Old: "HH-P1", New: "WEB-P2", ToStore: "01STORE0000000000000000002", ToStoreName: "team-b"}),
		)
	})
}

// rulesPairAt は、rulesPair の B の時計を bStart から始めたもの。
func rulesPairAt(t *testing.T, bStart time.Time) (port.Remote, machine, machine) {
	t.Helper()
	remote := newBareRemote(t)
	a := noticed(t, rulesFirstMachine(t, remote), "taro")
	b := noticed(t, cloneMachine(t, remote, "B", bStart), "hanako")
	b.sync(t, remote)
	return remote, a, b
}

// A が「Web」(HH-P1)を別の Store「team-b」へ移し、B が同時に「メイン」で「Web」の専用ルールの名前を変える。移動の
// 専用ルールの削除は削除と変更の重なりに数えないので、取り消しも修復 (9) の保存もない。A・B・A の順に同期すると、
// B(名前を変えた人)に移動の通知が 1 行だけ届き、A(移動した人)には何も届かない。移動の時刻が名前の変更より前でも
// あとでも同じ。
func TestRepairRulesMovedStoreRename(t *testing.T) {
	for name, bStart := range map[string]time.Time{
		"移動が先":  scenarioStart.Add(time.Hour),
		"移動があと": scenarioStart,
	} {
		t.Run(name, func(t *testing.T) {
			remote, a, b := rulesPairAt(t, bStart)
			moveWebToStoreAs(t, a, "taro")
			renameOwnRuleAs(t, b, "hanako")
			a.sync(t, remote)
			b.sync(t, remote)
			a.sync(t, remote)
			sameFiles(t, a, b)
			wantRepair(t, b, "HH-P1 は taro が team-b へ移動したため、あなたの変更は反映されませんでした。移動先は WEB-P2 です")
			wantNoRepair(t, a)
			for _, m := range []machine{a, b} {
				wantNoWebRules(t, m)
				if n := subjectCount(t, m.dir, orphanWebOp); n != 0 {
					t.Errorf("%s: %d commits %q, want none", m.dir, n, orphanWebOp)
				}
			}
		})
	}
}
