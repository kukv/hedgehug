//go:build !e2e

package main

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// 削除された値を指す設定の修復 (10) と、その通知を、本物の gitstore と repairing.Run と state.db の受信箱で、2 台の
// マシンの場面として確かめる(C5b2)。A のログイン名は taro(表示名 太郎)、B は hanako(表示名 花子)。ワークスペース
// 「メイン」とプロジェクト「Web」は rulesFirstMachine のもの。どの同期もエラーにならない(m.sync が確かめる。C5b2-15)。

const (
	reviewItem item.ID   = "01VALUESITEM00000000000003"
	reviewRule change.ID = "01RULE00000000000000REVIEW"
)

var (
	mainTodo  = ruleValue(mainValues, 1)
	mainDoing = ruleValue(mainValues, 2)
)

// valuesPair は、rulesFirstMachine の A が、メンバーの表示名(太郎・花子)と、「Web」のアイテム HH-3(ステータス
// 「Review」)を書いて同期したあと、remote を写した B。どちらにも通知の判定を付ける。
func valuesPair(t *testing.T) (port.Remote, machine, machine) {
	t.Helper()
	remote := newBareRemote(t)
	a := noticed(t, rulesFirstMachine(t, remote), "taro")
	it := item.Item{
		ID:    reviewItem,
		Place: item.Place{Workspace: mainWS, Project: webPR, Number: 3},
		Kind:  item.Epic, Rank: rank.First, Title: "レビュー待ち",
		Status: mainReview, Priority: ruleValue(mainValues, 8),
	}
	a.writeAs(t, "taro", "アイテムを作成", func(tx port.Tx) error {
		return errors.Join(
			tx.WriteMembers().Append(member.Rename("taro", "taro", "太郎"), member.Rename("hanako", "hanako", "花子")),
			tx.WriteWorkspaces().Append(mainWS, numbering.Draft(numbering.Items, 3)),
			tx.WriteItems().Append(reviewItem, item.Create(it)),
		)
	})
	a.sync(t, remote)
	b := noticed(t, cloneMachine(t, remote, "B", scenarioStart.Add(time.Hour)), "hanako")
	b.sync(t, remote)
	return remote, a, b
}

// setMainAs は、login の操作として、「メイン」の項目 f を from から to にする。
func setMainAs(t *testing.T, m machine, login, summary, f string, from, to any) {
	t.Helper()
	m.writeAs(t, login, summary, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(mainWS, change.NewSet(f, from, to))
	})
}

// deleteReviewAs は、login の操作として、app/workspaces の DeleteValue と同じ行で「メイン」の「Review」を削除する:
// 値の delete、「Review」のアイテムを初期ステータスにする set status、ワークフローとルールとプロジェクト別のルールの追従。
func deleteReviewAs(t *testing.T, m machine, login string) {
	t.Helper()
	log := m.wsLog(t, mainWS)
	st, _ := workspace.SettingsFromLog(log)
	list := workspace.ParseList("statuses")
	its := m.items(t, webPR)
	m.writeAs(t, login, "ステータス「Review」を削除", func(tx port.Tx) error {
		for _, it := range its {
			if it.Status != mainReview {
				continue
			}
			if err := tx.WriteItems().Append(it.ID, change.NewSet("status", mainReview, st.InitialStatus)); err != nil {
				return err
			}
		}
		ds := []change.Draft{change.NewDelete(list.Ref(mainReview))}
		ds = append(ds, workspace.WorkflowDeleteDrafts(st, mainReview)...)
		rds, _ := automation.StatusDeleteDrafts(automation.FromLog(log), mainReview, st.InitialStatus)
		ds = append(ds, rds...)
		ds = append(ds, projectrule.StatusDeleteDrafts(projectrule.FromLog(log), mainReview)...)
		return tx.WriteWorkspaces().Append(mainWS, ds...)
	})
}

// settings は、m の「メイン」の設定。
func (m machine) settings(t *testing.T) workspace.Settings {
	t.Helper()
	st, ok := workspace.SettingsFromLog(m.wsLog(t, mainWS))
	if !ok {
		t.Fatalf("%s: no settings of メイン", m.dir)
	}
	return st
}

// hasReview は、st に「Review」があるか。
func hasReview(st workspace.Settings) bool {
	return slices.ContainsFunc(st.Statuses, func(v workspace.Value) bool { return v.ID == mainReview })
}

// syncAll は、A・B・A の順に同期する。
func syncAll(t *testing.T, remote port.Remote, a, b machine) {
	t.Helper()
	a.sync(t, remote)
	b.sync(t, remote)
	a.sync(t, remote)
}

// A が「Review」を進行中の既定にし、B が同時に「Review」を削除する(HH-3 は Todo になる)。A・B・A の順に同期すると、
// B の修復が削除を取り消し、2 台とも「Review」があり、HH-3 は「Review」に戻る。B(削除した人)に 1 行、A には作らない。
func TestRepairValuesDefaultRestored(t *testing.T) {
	remote, a, b := valuesPair(t)
	setMainAs(t, a, "taro", "進行中の既定を変更", "category_default/doing", mainDoing, mainReview)
	deleteReviewAs(t, b, "hanako")
	if got := b.item(t, reviewItem).Status; got != mainTodo {
		t.Fatalf("HH-3 on B after the delete = %s, want Todo", got)
	}
	syncAll(t, remote, a, b)
	sameFiles(t, a, b)
	for _, m := range []machine{a, b} {
		st := m.settings(t)
		if !hasReview(st) || st.CategoryDefaults[workspace.CategoryDoing] != mainReview {
			t.Errorf("%s: Review is gone or not the default of doing: %+v", m.dir, st.CategoryDefaults)
		}
		if got := m.item(t, reviewItem).Status; got != mainReview {
			t.Errorf("HH-3 on %s = %s, want Review", m.dir, got)
		}
	}
	wantRepair(t, b, "「Review」は 太郎 が同時に既定にしたため、削除を取り消しました")
	wantNoRepair(t, a)
}

// addWebReviewRulesAs は、login の操作として、「Web」に Todo→Review のワークフローの制約と、きっかけ「Review」の専用
// ルール「レビューで通知」を足す。
func addWebReviewRulesAs(t *testing.T, m machine, login string) {
	t.Helper()
	rule := automation.Rule{
		ID: reviewRule, Name: "レビューで通知", Trigger: automation.TriggerStatus, TriggerStatus: mainReview,
		Action: automation.ActionAssignee, Value: "taro", Enabled: true, Scope: webPR,
	}
	m.writeAs(t, login, "プロジェクト「Web」のルールを変更", func(tx port.Tx) error {
		ds := []change.Draft{change.NewAdd(projectrule.WorkflowField(webPR, mainTodo), mainReview)}
		return tx.WriteWorkspaces().Append(mainWS, append(ds, automation.Drafts(automation.Rule{}, rule)...)...)
	})
}

// wantRepairs は、m の受信箱の「同期の修復」が、差出人が空(hedgehug)で本文が texts(順は問わない)の行だけなことを確かめる。
func wantRepairs(t *testing.T, m machine, texts ...string) {
	t.Helper()
	rows := m.notices(t, port.NoticeRepair)
	var got []string
	for _, r := range rows {
		if r.Sender != "" {
			t.Errorf("repair row on %s has sender %q", m.dir, r.Sender)
		}
		got = append(got, r.Text)
	}
	slices.Sort(got)
	want := slices.Sorted(slices.Values(texts))
	if !slices.Equal(got, want) {
		t.Errorf("repair rows on %s = %q, want %q", m.dir, got, want)
	}
}

// A が「Web」に Todo→Review のワークフローの制約と、きっかけ「Review」のルールを足し、B が同時に「Review」を削除する。
// A・B・A の順に同期すると、B の修復が制約から「Review」を外し、ルールを無効にする。A(足した人)に 1 行ずつ、B には作らない。
func TestRepairValuesRules(t *testing.T) {
	remote, a, b := valuesPair(t)
	addWebReviewRulesAs(t, a, "taro")
	if tos := projectrule.FromLog(a.wsLog(t, mainWS)).Of(webPR).Workflow[mainTodo]; !slices.Contains(tos, mainReview) {
		t.Fatalf("the workflow of Web on A before the repair: Todo -> %v, want Review in it", tos)
	}
	deleteReviewAs(t, b, "hanako")
	syncAll(t, remote, a, b)
	sameFiles(t, a, b)
	for _, m := range []machine{a, b} {
		if hasReview(m.settings(t)) {
			t.Errorf("%s: Review is not deleted", m.dir)
		}
		log := m.wsLog(t, mainWS)
		for from, tos := range projectrule.FromLog(log).Of(webPR).Workflow {
			if from == mainReview || slices.Contains(tos, mainReview) {
				t.Errorf("%s: the workflow of Web still has Review: %s -> %v", m.dir, from, tos)
			}
		}
		i := slices.IndexFunc(automation.FromLog(log), func(r automation.Rule) bool { return r.ID == reviewRule })
		if i < 0 || automation.FromLog(log)[i].Enabled {
			t.Errorf("%s: the rule レビューで通知 is missing or still enabled", m.dir)
		}
	}
	wantRepairs(t, a,
		"プロジェクト「Web」のワークフローの制約から、花子 が「Review」を削除したため、「Review」を外しました",
		"自動化「レビューで通知」は、花子 が「Review」を削除したため、無効にしました",
	)
	wantNoRepair(t, b)
}

// A が「Review」を完了の分類にし、B が同時に「Review」を初期ステータスにする。A・B・A の順に同期すると、B の修復が初期
// ステータスを「Todo」に戻す。B(初期ステータスにした人)に 1 行、A には作らない。
func TestRepairValuesFinishedInitial(t *testing.T) {
	remote, a, b := valuesPair(t)
	setMainAs(t, a, "taro", "ステータス「Review」を変更", "statuses/"+string(mainReview)+"/category", workspace.CategoryDoing, workspace.CategoryDone)
	setMainAs(t, b, "hanako", "初期ステータスを変更", "initial_status", mainTodo, mainReview)
	syncAll(t, remote, a, b)
	sameFiles(t, a, b)
	for _, m := range []machine{a, b} {
		if got := m.settings(t).InitialStatus; got != mainTodo {
			t.Errorf("initial status on %s = %s, want Todo", m.dir, got)
		}
	}
	wantRepair(t, b, "「Review」は 太郎 が完了の分類にしたため、初期ステータスを「Todo」に戻しました")
	wantNoRepair(t, a)
}

// A が「Web」で「Review」を使わないにし、B が同時に「Review」を初期ステータスにする。A・B・A の順に同期しても、外された
// 初期ステータスは直さない(C5b2-14): 同期の修復の保存はなく、通知もない。「Web」の使わないステータスに「Review」が残る。
func TestRepairValuesHiddenInitial(t *testing.T) {
	remote, a, b := valuesPair(t)
	hideReviewAs(t, a, "taro")
	setMainAs(t, b, "hanako", "初期ステータスを変更", "initial_status", mainTodo, mainReview)
	syncAll(t, remote, a, b)
	sameFiles(t, a, b)
	for _, m := range []machine{a, b} {
		// Store のどのログ(ワークスペース・プロジェクト・アイテムなど)にも、同期の修復の行がない。
		for path, body := range files(t, m.dir) {
			if strings.Contains(body, `"actor":"`+string(change.ActorSync)+`"`) {
				t.Errorf("%s: %s has a repair row by sync", m.dir, path)
			}
		}
		log := m.wsLog(t, mainWS)
		if got := m.settings(t).InitialStatus; got != mainReview {
			t.Errorf("initial status on %s = %s, want Review", m.dir, got)
		}
		if hidden := projectrule.FromLog(log).Of(webPR).Hidden; !slices.Contains(hidden, mainReview) {
			t.Errorf("hidden statuses of Web on %s = %v, want Review in them", m.dir, hidden)
		}
		wantNoRepair(t, m)
	}
}
