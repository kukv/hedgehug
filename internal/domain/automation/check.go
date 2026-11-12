package automation

import (
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// SwitchRefusal は、スイッチを ON にするときの断り(03「ルールの一覧」の下)。
const SwitchRefusal = "きっかけと同じステータスには変更できません。ルールを編集してください"

const sameStatus = "きっかけと同じステータスには変更できません"

// Normalize は、保存する形にする。名前の前後の空白を除き、きっかけで使わない項目を空にする
// (03「きっかけを他のものに変えると、条件の選択は保存しない」)。
func Normalize(r Rule) Rule {
	r.Name = workspace.NormalizeName(r.Name)
	if r.Trigger != TriggerStatus {
		r.TriggerStatus = ""
	}
	if r.Trigger != TriggerPRMerged {
		r.AllMerged = false
	}
	return r
}

// Valid は、きっかけ・すること・値の参照が正しいか。false は ErrNotFound にする(B4-6)。logins は Store のメンバー。
func Valid(s workspace.Settings, logins []string, r Rule) bool {
	if !slices.Contains(Triggers, r.Trigger) || (r.Trigger == TriggerStatus && !hasStatus(s, r.TriggerStatus)) {
		return false
	}
	switch r.Action {
	case ActionStatus:
		return hasStatus(s, change.ID(r.Value))
	case ActionAssignee:
		return r.Value == Unassigned || slices.ContainsFunc(logins, func(l string) bool { return strings.EqualFold(l, r.Value) })
	}
	return false
}

func hasStatus(s workspace.Settings, id change.ID) bool {
	return slices.ContainsFunc(s.Statuses, func(v workspace.Value) bool { return v.ID == id })
}

// Check は、保存の条件(03 の表の順)。r は Normalize のあと、rules はいまのルール。
func Check(rules []Rule, r Rule) error {
	if r.Name == "" {
		return refusal.New("名前を入力してください")
	}
	for _, o := range rules {
		if o.ID != r.ID && o.Name == r.Name {
			return refusal.New("同じ名前のルールがあります")
		}
	}
	return CheckSameStatus(r)
}

// CheckSameStatus は、きっかけのステータスへ変えるルールを断る(表の最後の行)。
func CheckSameStatus(r Rule) error {
	if r.Trigger == TriggerStatus && r.Action == ActionStatus && string(r.TriggerStatus) == r.Value {
		return refusal.New(sameStatus)
	}
	return nil
}
