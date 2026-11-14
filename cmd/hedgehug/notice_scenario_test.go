//go:build !e2e

package main

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/workspaces"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/comment"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// 同期で受け取った保存からの通知を、本物の gitstore と state.db と noticing.Service で、2 台のマシンの場面として
// 確かめる(決定 C4b1-17)。A のログイン名は taro、B は hanako。

// noticePair は、同じ remote から始め、通知を判定する A(taro)と B(hanako)。
func noticePair(t *testing.T) (port.Remote, machine, machine) {
	t.Helper()
	remote := newBareRemote(t)
	a := noticed(t, firstMachine(t, remote), "taro")
	b := noticed(t, cloneMachine(t, remote, "B", scenarioStart.Add(time.Hour)), "hanako")
	b.sync(t, remote)
	return remote, a, b
}

// setTask は、サンプルのタスク HH-2 に行を足す。
func setTask(ds ...change.Draft) func(tx port.Tx) error {
	return func(tx port.Tx) error { return tx.WriteItems().Append(apptest.SampleTask, ds...) }
}

// A がアイテムの担当者を hanako にして同期すると、B の受信箱の「割り当て」に 1 行。B がもう一度同期しても 1 行のまま。
func TestNoticeScenarioAssignOnce(t *testing.T) {
	remote, a, b := noticePair(t)
	a.writeAs(t, "taro", "担当者を変更", setTask(change.NewSet("assignee", nil, "hanako")))
	a.sync(t, remote)
	b.sync(t, remote)
	rows := b.notices(t, port.NoticeAssign)
	if len(rows) != 1 || rows[0].Item != apptest.SampleTask || rows[0].Text != "あなたに割り当てました" || rows[0].Sender != "taro" {
		t.Fatalf("assign rows on B = %+v, want one for HH-2 from taro", rows)
	}
	b.sync(t, remote)
	if again := b.notices(t, port.NoticeAssign); len(again) != 1 {
		t.Errorf("assign rows on B after sync again = %+v, want one", again)
	}
	if onA := a.notices(t); len(onA) != 0 {
		t.Errorf("rows on A = %+v, want none", onA)
	}
}

const scenarioComment change.ID = "01SCENECOMMENT000000000001"

// A がコメント「@hanako 見て」を投稿して同期すると、B にメンションが 1 行。B が読んだあと、A が本文を「@hanako 直した」に
// 編集して同期すると、B の行は 1 つのまま本文が変わり、未読に戻る。
func TestNoticeScenarioMention(t *testing.T) {
	remote, a, b := noticePair(t)
	a.writeAs(t, "taro", "HH-2 にコメント", setTask(change.NewComment(scenarioComment, nil, comment.Value("@hanako 見て"))))
	a.sync(t, remote)
	b.sync(t, remote)
	rows := b.notices(t, port.NoticeMention)
	if len(rows) != 1 || rows[0].Text != "@hanako 見て" || rows[0].Sender != "taro" || rows[0].Read {
		t.Fatalf("mention rows on B = %+v, want one unread", rows)
	}
	if err := b.inbox.SetRead(t.Context(), rows[0].ID, true); err != nil {
		t.Fatal(err)
	}
	a.writeAs(t, "taro", "コメントを編集", setTask(change.NewComment(scenarioComment, comment.Value("@hanako 見て"), comment.Value("@hanako 直した"))))
	a.sync(t, remote)
	b.sync(t, remote)
	got := b.notices(t, port.NoticeMention)
	if len(got) != 1 || got[0].ID != rows[0].ID || got[0].Text != "@hanako 直した" || got[0].Read {
		t.Errorf("mention rows on B after edit = %+v, want the same row unread with the new text", got)
	}
}

// B がウォッチしているアイテムのステータスを A が Done に変えて同期すると、B に「ステータスを Done に変更」。
func TestNoticeScenarioWatch(t *testing.T) {
	remote, a, b := noticePair(t)
	b.writeAs(t, "hanako", "ウォッチ", setTask(change.NewAdd("watchers", "hanako")))
	b.sync(t, remote)
	a.sync(t, remote)
	a.writeAs(t, "taro", "ステータスを変更", setTask(change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusDone)))
	a.sync(t, remote)
	b.sync(t, remote)
	rows := b.notices(t, port.NoticeWatch)
	if len(rows) != 1 || rows[0].Item != apptest.SampleTask || rows[0].Text != "ステータスを Done に変更" || rows[0].Sender != "taro" {
		t.Errorf("watch rows on B = %+v, want one for the status", rows)
	}
}

const (
	scenarioReview change.ID = "01SCENEREVIEW0000000000001"
	scenarioRule   change.ID = "01SCENERULE000000000000001"
)

var scenarioStatuses = workspace.List{Kind: workspace.ListStatuses}

// A がルールのきっかけにしたステータスを削除して同期すると、B に自動化の 1 行。A の受信箱は A の操作の 1 行だけ。
// B が何か保存して同期し、A がもう一度同期しても、A の受信箱は自動化の 1 行のまま(Review Focus 2)。
func TestNoticeScenarioStatusDelete(t *testing.T) {
	remote, a, b := noticePair(t)
	review := workspace.Value{ID: scenarioReview, Name: "Review", Color: "blue", Category: workspace.CategoryDoing, Rank: "Z"}
	rule := automation.Rule{
		ID: scenarioRule, Name: "レビューへ", Trigger: automation.TriggerStatus, TriggerStatus: scenarioReview,
		Action: automation.ActionStatus, Value: string(apptest.SampleStatusDone), Enabled: true,
	}
	a.writeAs(t, "taro", "ルールを作成", func(tx port.Tx) error {
		ds := append(workspace.ValueDrafts(scenarioStatuses, review), automation.Drafts(automation.Rule{}, rule)...)
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, ds...)
	})
	a.sync(t, remote)
	b.sync(t, remote)

	svc := workspaces.Service{Stores: a.stores, IDs: &letterIDs{letter: "W"}, Identity: apptest.Identity{Login: "taro"}, Inbox: a.inbox}
	if err := svc.DeleteValue(t.Context(), a.id, apptest.SampleWorkspace, scenarioStatuses, scenarioReview, apptest.SampleStatusInProgress); err != nil {
		t.Fatal(err)
	}
	a.sync(t, remote)
	b.sync(t, remote)
	want := automation.RewrittenText("レビューへ", "Review", automation.PartTrigger, "InProgress")
	rows := b.notices(t, port.NoticeAutomation)
	if len(rows) != 1 || rows[0].Rule != scenarioRule || rows[0].Text != want || rows[0].Sender != "" {
		t.Errorf("automation rows on B = %+v, want one %q", rows, want)
	}
	if onA := a.notices(t); len(onA) != 1 || onA[0].Kind != port.NoticeAutomation || onA[0].Text != want {
		t.Fatalf("rows on A = %+v, want only the one of A's own delete", onA)
	}

	// A への通知にならない保存(taro は HH-2 をウォッチしているので、タイトルやプロパティの変更はウォッチの通知になる)
	b.writeAs(t, "hanako", "ウォッチ", setTask(change.NewAdd("watchers", "hanako")))
	b.sync(t, remote)
	a.sync(t, remote)
	if onA := a.notices(t); len(onA) != 1 || onA[0].Kind != port.NoticeAutomation {
		t.Errorf("rows on A after B's save = %+v, want only the one automation row", onA)
	}
}

// A の保存のあとで C(hanako)が clone すると、C の最初の同期では clone より前の保存から通知を作らない。
// そのあとの A の保存からは作る。
func TestNoticeScenarioCloneBaseline(t *testing.T) {
	remote := newBareRemote(t)
	a := noticed(t, firstMachine(t, remote), "taro")
	a.writeAs(t, "taro", "担当者を変更", setTask(change.NewSet("assignee", nil, "hanako")))
	a.sync(t, remote)
	c := noticed(t, cloneMachine(t, remote, "C", scenarioStart.Add(time.Hour)), "hanako")
	c.sync(t, remote)
	if rows := c.notices(t); len(rows) != 0 {
		t.Fatalf("rows on C after the first sync = %+v, want none", rows)
	}
	a.writeAs(t, "taro", "コメント", setTask(change.NewComment(scenarioComment, nil, comment.Value("@hanako よろしく"))))
	a.sync(t, remote)
	c.sync(t, remote)
	if rows := c.notices(t); len(rows) != 1 || rows[0].Kind != port.NoticeMention {
		t.Errorf("rows on C after A's comment = %+v, want one mention", rows)
	}
}
