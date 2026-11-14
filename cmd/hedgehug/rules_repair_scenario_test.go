//go:build !e2e

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
	"github.com/kukv/hedgehug/internal/infra/gitstore"
	"github.com/kukv/hedgehug/internal/infra/gitstore/filemerge"
)

// なくなったプロジェクトのルールの修復 (9) と、完全に削除したプロジェクトのルールの行の取り除きと、その通知を、本物の
// gitstore と repairing.Run と state.db の受信箱で、2 台のマシンの場面として確かめる(C5b1)。A のログイン名は taro、
// B は hanako。サンプルのワークスペース「Web」(HH)と同じ接頭辞を使わないよう、サンプルを作らない Store で始める。

const (
	mainWS      workspace.ID = "01RULESWS0000000000000MAIN"
	subWS       workspace.ID = "01RULESWS00000000000000SUB"
	webPR       project.ID   = "01RULESPR00000000000000WEB"
	webOwnRule  change.ID    = "01RULE000000000000000OWN01"
	webCarried  change.ID    = "01RULE000000000000CARRIED1"
	mainValues               = "01MAINVALUE"
	subValues                = "01SUBVALUES"
	orphanWebOp              = "プロジェクト「Web」のルールを削除(プロジェクトがない)"
)

// ruleValue は、値の内部 ID の接頭辞 prefix の n 番目(DefaultSettings が振る順。3 は Done)。17 は「メイン」の Review。
func ruleValue(prefix string, n int) change.ID {
	return change.ID(fmt.Sprintf("%s%015d", prefix, n))
}

var mainReview = ruleValue(mainValues, 17)

// ruleSettings は、既定の設定(値の内部 ID の接頭辞 prefix)。review が空でなければ、ステータス「Review」を末尾に足す。
func ruleSettings(prefix string, review change.ID) workspace.Settings {
	n := 0
	s := workspace.DefaultSettings(func() change.ID {
		n++
		return ruleValue(prefix, n)
	})
	if review != "" {
		last := s.Statuses[len(s.Statuses)-1].Rank
		s.Statuses = append(s.Statuses, workspace.Value{ID: review, Name: "Review", Color: "blue", Category: workspace.CategoryDoing, Rank: rank.After(last)})
	}
	return s
}

// rulesFirstMachine は、メンバー(taro・hanako)、ワークスペース「メイン」(HH。ステータス「Review」あり)と「サブ」(SB)、
// 「メイン」のプロジェクト「Web」(HH-P1)と、「Web」の専用ルール「完了で閉じる」を保存して、空の remote へ最初に送ったマシン。
func rulesFirstMachine(t *testing.T, remote port.Remote) machine {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "a")
	if err := gitstore.Create(dir, "01STORE0000000000000000001", scenarioStart); err != nil {
		t.Fatal(err)
	}
	m := newMachine(t, dir, "A", scenarioStart)
	main, err := workspace.CreateWithSettings(workspace.Workspace{ID: mainWS, Name: "メイン", Prefix: "HH"}, ruleSettings(mainValues, mainReview))
	if err != nil {
		t.Fatal(err)
	}
	sub, err := workspace.CreateWithSettings(workspace.Workspace{ID: subWS, Name: "サブ", Prefix: "SB"}, ruleSettings(subValues, ""))
	if err != nil {
		t.Fatal(err)
	}
	web := project.Project{ID: webPR, Place: project.Place{Workspace: mainWS, Number: 1}, Rank: "V", Title: "Web", Owner: "taro", Status: project.Todo}
	own := automation.Rule{
		ID: webOwnRule, Name: "完了で閉じる", Trigger: automation.TriggerPRMerged, Action: automation.ActionStatus,
		Value: string(ruleValue(mainValues, 3)), Enabled: true, Scope: webPR,
	}
	m.writeAs(t, "taro", "ワークスペースを作成", func(tx port.Tx) error {
		return errors.Join(
			tx.WriteMembers().Append(member.Init(), member.Join("taro"), member.Join("hanako")),
			tx.WriteWorkspaces().Append(mainWS, append([]change.Draft{main, numbering.Draft(numbering.Projects, 1)}, automation.Drafts(automation.Rule{}, own)...)...),
			tx.WriteWorkspaces().Append(subWS, sub),
			tx.WriteProjects().Append(web.ID, project.Create(web)),
		)
	})
	m.sync(t, remote)
	return m
}

// rulesPair は、rulesFirstMachine の A(taro)と、remote を写した B(hanako)。どちらにも通知の判定を付ける。
func rulesPair(t *testing.T) (port.Remote, machine, machine) {
	t.Helper()
	remote := newBareRemote(t)
	a := noticed(t, rulesFirstMachine(t, remote), "taro")
	b := noticed(t, cloneMachine(t, remote, "B", scenarioStart.Add(time.Hour)), "hanako")
	b.sync(t, remote)
	return remote, a, b
}

// wsLog は、m のワークスペース ws のログ。
func (m machine) wsLog(t *testing.T, ws workspace.ID) []change.Record {
	t.Helper()
	var log []change.Record
	err := m.stores.Read(t.Context(), m.id, func(r port.Reader) error {
		var err error
		log, err = r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return log
}

// moveWebAs は、login の操作として、「Web」を「サブ」の番号 1 へ移す: プロジェクトの set place と、「サブ」の番号の行と、
// transfer.CarryRules の移動先の行を「サブ」に、移動元の行を「メイン」に書く。専用ルールの新しい内部 ID は webCarried。
func moveWebAs(t *testing.T, m machine, login string) {
	t.Helper()
	mainLog, subLog := m.wsLog(t, mainWS), m.wsLog(t, subWS)
	from, _ := workspace.SettingsFromLog(mainLog)
	to, _ := workspace.SettingsFromLog(subLog)
	own := slices.DeleteFunc(automation.FromLog(mainLog), func(r automation.Rule) bool { return r.Scope != webPR })
	carried := transfer.CarryRules(transfer.CarryInput{
		Project: webPR, From: from, To: to, Rules: projectrule.FromLog(mainLog).Of(webPR), Own: own,
		NewID: func() change.ID { return webCarried },
	})
	old := project.Place{Workspace: mainWS, Number: 1}
	m.writeAs(t, login, "プロジェクト「Web」を メイン から サブ へ移動", func(tx port.Tx) error {
		return errors.Join(
			tx.WriteProjects().Append(webPR, change.NewSet("place", old, project.Place{Workspace: subWS, Number: 1})),
			tx.WriteWorkspaces().Append(subWS, append([]change.Draft{numbering.Draft(numbering.Projects, 1)}, carried.Destination...)...),
			tx.WriteWorkspaces().Append(mainWS, carried.Source...),
		)
	})
}

// hideReviewAs は、login の操作として、「メイン」で「Web」の使わないステータスに「Review」を足す。
func hideReviewAs(t *testing.T, m machine, login string) {
	t.Helper()
	m.writeAs(t, login, "使わないステータスを変更", func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(mainWS, change.NewAdd(projectrule.HiddenField(webPR), mainReview))
	})
}

// purgeProjectAs は、本物のプロジェクトの「完全に削除」と同じく、projects/<P> を消し、ワークスペース ws のログから P の
// ルールの行(project_rules/<P>/… と専用ルール)を除いて書き直し、purged.jsonl に entity が project で display の行を
// 足して、login の 1 つのコミットにする(purgeSprintAs と同じ作り)。
func purgeProjectAs(t *testing.T, m machine, login string, ws workspace.ID, p project.ID, display string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(m.dir, "projects", string(p))); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(m.dir, "workspaces", string(ws), "log.jsonl")
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, filemerge.DropPurgedRules(log, map[string]bool{string(p): true}), 0o600); err != nil {
		t.Fatal(err)
	}
	line, err := json.Marshal(map[string]string{
		"id": "01PURGEPROJECT000000000001", "at": scenarioStart.Format(change.TimeLayout), "by": login,
		"actor": "user", "entity": "project", "target": string(p), "display": display,
	})
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(m.dir, "purged.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	commitAll(t, m.dir, login, "完全に削除")
}

// wantNoWebRules は、m の「メイン」に、「Web」のプロジェクト別のルールも専用ルールもないことを確かめる。
func wantNoWebRules(t *testing.T, m machine) {
	t.Helper()
	log := m.wsLog(t, mainWS)
	if rules := projectrule.FromLog(log).Of(webPR); !rules.Empty() {
		t.Errorf("project rules of Web on %s = %+v, want none", m.dir, rules)
	}
	for _, r := range automation.FromLog(log) {
		if r.Scope == webPR {
			t.Errorf("own rule of Web on %s: %+v", m.dir, r)
		}
	}
}

// A が「Web」を「サブ」へ移し、B が同時に「メイン」で「Web」の使わないステータスに「Review」を足す。A が同期し、B が
// 同期し、A がもう一度同期すると、B の修復 (9) が「メイン」の「Web」のルールを外す。「サブ」の「Web」の専用ルールは移動で
// 引き継いだまま。2 台は同じで、どちらにも同期の修復の通知はない(「Web」は同じ Store にある)。修復の保存は 1 つ。
func TestRepairRulesMovedWorkspace(t *testing.T) {
	remote, a, b := rulesPair(t)
	moveWebAs(t, a, "taro")
	hideReviewAs(t, b, "hanako")
	a.sync(t, remote)
	b.sync(t, remote)
	a.sync(t, remote)
	sameFiles(t, a, b)
	for _, m := range []machine{a, b} {
		wantNoWebRules(t, m)
		subLog := m.wsLog(t, subWS)
		own := slices.DeleteFunc(automation.FromLog(subLog), func(r automation.Rule) bool { return r.Scope != webPR })
		if len(own) != 1 || own[0].ID != webCarried || own[0].Name != "完了で閉じる" || own[0].Value != string(ruleValue(subValues, 3)) {
			t.Errorf("own rules of Web in サブ on %s = %+v, want the carried one", m.dir, own)
		}
		if rules := projectrule.FromLog(subLog).Of(webPR); !rules.Empty() {
			t.Errorf("project rules of Web in サブ on %s = %+v, want none", m.dir, rules)
		}
		if n := subjectCount(t, m.dir, orphanWebOp); n != 1 {
			t.Errorf("%s: %d commits %q, want 1", m.dir, n, orphanWebOp)
		}
		wantNoRepair(t, m)
	}
}

// A が「Web」(HH-P1)を完全に削除し、B が同時に「Web」の使わないステータスに「Review」を足す。A が同期し、B が同期すると、
// B の合わせが「Web」のルールの行を取り除き、B(足した人)の受信箱に 1 行。A が同期したあと、2 台のログに「Web」のルールの
// 行はない。A には作らない。
func TestRepairRulesPurgedProject(t *testing.T) {
	remote, a, b := rulesPair(t)
	purgeProjectAs(t, a, "taro", mainWS, webPR, "HH-P1")
	hideReviewAs(t, b, "hanako")
	a.sync(t, remote)
	b.sync(t, remote)
	wantRepair(t, b, "HH-P1 は完全に削除されたため、あなたの変更は反映されませんでした")
	a.sync(t, remote)
	sameFiles(t, a, b)
	log := "workspaces/" + string(mainWS) + "/log.jsonl"
	for _, m := range []machine{a, b} {
		got := files(t, m.dir)[log]
		for _, s := range []string{"project_rules/" + string(webPR), string(webOwnRule)} {
			if strings.Contains(got, s) {
				t.Errorf("%s: the log still has %s", m.dir, s)
			}
		}
	}
	wantNoRepair(t, a)
}
