// Package automation は、自動化のルール(仕様 03「自動化」)の型・保存の行・検査・実行の規則を持つ。
// ルールはワークスペースの log.jsonl の項目 rules/<ID>/<項目> に持つ(B4-2)。
package automation

import (
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// Trigger は、きっかけ(03「きっかけとすること」の 6 つ。B4-4)。
type Trigger string

const (
	TriggerPROpen      Trigger = "pr_open"          // PR がリンクされて Open のとき
	TriggerReview      Trigger = "review_requested" // レビューが依頼されたとき
	TriggerPRMerged    Trigger = "pr_merged"        // リンクした PR がマージされたとき
	TriggerIssueClosed Trigger = "issue_closed"     // リンクした Issue が閉じたとき
	TriggerStatus      Trigger = "status"           // ステータスが次になったとき
	TriggerStepsDone   Trigger = "steps_done"       // Step がすべて完了したとき
)

// Triggers は、きっかけの並び(作成ダイアログの選択肢の順)。
var Triggers = []Trigger{TriggerPROpen, TriggerReview, TriggerPRMerged, TriggerIssueClosed, TriggerStatus, TriggerStepsDone}

// Action は、すること。
type Action string

const (
	ActionStatus   Action = "status"   // ステータスを変更
	ActionAssignee Action = "assignee" // 担当者を変更
)

// Unassigned は、担当者を「未割り当てにする」の値。
const Unassigned = "@none"

// Rule は、自動化のルール 1 つ。比べられるように、値の型だけで持つ。
type Rule struct {
	ID            change.ID
	Name          string
	Trigger       Trigger
	TriggerStatus change.ID // TriggerStatus のときだけ
	AllMerged     bool      // TriggerPRMerged のときだけ。「リンク中の PR がすべてマージ済みのときだけ」
	Action        Action
	Value         string // ステータスの ID、メンバーのログイン名、Unassigned
	Enabled       bool
	Scope         project.ID // 空ならワークスペース全体。あれば、そのプロジェクトだけのルール(B5a-10)
}

const prefix = "rules/"

// Field は、ルール id の項目名 rules/<id>/<name>。
func Field(id change.ID, name string) string { return prefix + string(id) + "/" + name }
