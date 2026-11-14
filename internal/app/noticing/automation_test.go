package noticing_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	statusReview change.ID = "01REVIEW000000000000000001"
	ruleID       change.ID = "01RULE00000000000000000001"
)

var statuses = workspace.List{Kind: workspace.ListStatuses}

// reviewRule は、ルール「レビューへ」(ステータスが Review になったとき、ステータスを value に変更)。
func reviewRule(value change.ID) automation.Rule {
	return automation.Rule{
		ID: ruleID, Name: "レビューへ", Trigger: automation.TriggerStatus, TriggerStatus: statusReview,
		Action: automation.ActionStatus, Value: string(value), Enabled: true,
	}
}

func appendWorkspace(ds ...change.Draft) func(tx port.Tx) error {
	return func(tx port.Tx) error { return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, ds...) }
}

// prepareReview は、ステータス Review とルール rule を足す(判定しない保存)。
func (f *fixture) prepareReview(rule automation.Rule) {
	f.t.Helper()
	v := workspace.Value{ID: statusReview, Name: "Review", Color: "blue", Category: workspace.CategoryDoing, Rank: "Z"}
	f.save(user("taro"), appendWorkspace(append(workspace.ValueDrafts(statuses, v), automation.Drafts(automation.Rule{}, rule)...)...))
}

// deleteReview は、ステータス Review を削除して InProgress に置き換える行(DeleteValue と同じ、ルールの書き換えを含む)。
func deleteReview(rule automation.Rule) []change.Draft {
	ds, _ := automation.StatusDeleteDrafts([]automation.Rule{rule}, statusReview, apptest.SampleStatusInProgress)
	return append([]change.Draft{change.NewDelete(statuses.Ref(statusReview))}, ds...)
}

func TestAutomationFromStatusDelete(t *testing.T) {
	rewritten := reviewRule(apptest.SampleStatusDone)
	f := setup(t)
	f.prepareReview(rewritten)
	f.judge(f.save(user("taro"), appendWorkspace(deleteReview(rewritten)...)))
	n := f.wantOne()
	want := port.Notice{
		Kind: port.NoticeAutomation, Store: store, Workspace: apptest.SampleWorkspace, Rule: ruleID,
		Text: automation.RewrittenText("レビューへ", "Review", "きっかけ", "InProgress"),
	}
	if n != want {
		t.Errorf("notice = %+v, want %+v", n, want)
	}

	// きっかけと同じステータスへの変更になって無効にした
	disabled := reviewRule(apptest.SampleStatusInProgress)
	g := setup(t)
	g.prepareReview(disabled)
	g.judge(g.save(user("taro"), appendWorkspace(deleteReview(disabled)...)))
	if n := g.wantOne(); n.Kind != port.NoticeAutomation || n.Text != automation.DisabledByDeleteText("レビューへ") || n.Item != "" {
		t.Errorf("notice = %+v", n)
	}

	// ルールが変わらない削除
	other := reviewRule(apptest.SampleStatusDone)
	other.TriggerStatus = apptest.SampleStatusPending
	h := setup(t)
	h.prepareReview(other)
	h.judge(h.save(user("taro"), appendWorkspace(deleteReview(other)...)))
	h.wantNone()
}

func TestAutomationSkipsReadOnlyAndHidden(t *testing.T) {
	lists := map[string][]port.StoreInfo{
		"閲覧のみ": {{ID: store, Name: "main", ReadOnly: true}},
		"非表示":  nil,
	}
	for name, infos := range lists {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			f.svc.List = apptest.StoreList{Stores: infos}
			rule := reviewRule(apptest.SampleStatusDone)
			f.prepareReview(rule)
			f.judge(f.save(user("taro"), func(tx port.Tx) error {
				if err := appendWorkspace(deleteReview(rule)...)(tx); err != nil {
					return err
				}
				return setTask(assign("hanako"))(tx)
			}))
			if n := f.wantOne(); n.Kind != port.NoticeAssign {
				t.Errorf("notice = %+v, want assign only", n)
			}
		})
	}
}

// あなたの別のマシンでの削除も、このマシンの通知を作る(このマシンの削除は判定に入らない。C4b1-14)。
func TestAutomationFromOwnOtherMachine(t *testing.T) {
	rule := reviewRule(apptest.SampleStatusDone)
	f := setup(t)
	f.prepareReview(rule)
	f.judge(f.save(user("hanako"), appendWorkspace(deleteReview(rule)...)))
	if n := f.wantOne(); n.Kind != port.NoticeAutomation || n.Rule != ruleID || n.Sender != "" {
		t.Errorf("notice = %+v", n)
	}
}

// 別の Store から移ってきたワークスペースの行は、移動元の古い行なので作らない(C4b1-19 と同じ理由)。
func TestAutomationSkipsMovedInWorkspace(t *testing.T) {
	rule := reviewRule(apptest.SampleStatusDone)
	f := setup(t)
	f.prepareReview(rule)
	moved := transfer.NewMovedIn(transfer.MovedInInfo{Old: "MB", FromStore: "S2", FromStoreName: "mobile", FromWorkspace: apptest.SampleWorkspace})
	f.judge(f.save(user("taro"), appendWorkspace(append(deleteReview(rule), moved)...)))
	f.wantNone()
}

// 表示中の Store の一覧を読めないときは、Judge のエラーにする(このマシンの読み取りの失敗。C4b1-5)。
func TestAutomationStoreListError(t *testing.T) {
	rule := reviewRule(apptest.SampleStatusDone)
	f := setup(t)
	want := errors.New("一覧を読めない")
	f.svc.List = failingList{want}
	f.prepareReview(rule)
	if err := f.judgeErr(f.save(user("taro"), appendWorkspace(deleteReview(rule)...))); !errors.Is(err, want) {
		t.Errorf("Judge = %v, want %v", err, want)
	}
	f.wantNone()
}

type failingList struct{ err error }

func (l failingList) List(context.Context) ([]port.StoreInfo, error) { return nil, l.err }
