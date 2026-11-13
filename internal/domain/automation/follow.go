package automation

import "github.com/kukv/hedgehug/internal/domain/change"

// RuleChange は、ステータスの削除で変わったルール(通知の材料)。
type RuleChange struct {
	Rule     Rule   // 変えたあと
	Disabled bool   // きっかけと同じステータスへの変更になって無効にした
	Part     string // 書き換えた側(「きっかけ」か「すること」)。両方なら「すること」
}

// StatusDeleteDrafts は、ステータス id を repl に置き換えるルールの行と、変わったルール
// (03「値の編集」の「削除」の自動化)。書き換えてきっかけとすることが同じステータスになれば無効にする。
func StatusDeleteDrafts(rules []Rule, id, repl change.ID) ([]change.Draft, []RuleChange) {
	var ds []change.Draft
	var changes []RuleChange
	for _, r := range rules {
		n, part := r, ""
		if n.Trigger == TriggerStatus && n.TriggerStatus == id {
			n.TriggerStatus, part = repl, "きっかけ"
		}
		if n.Action == ActionStatus && n.Value == string(id) {
			n.Value, part = string(repl), "すること"
		}
		if part == "" {
			continue
		}
		disabled := CheckSameStatus(n) != nil
		if disabled {
			n.Enabled = false
		}
		ds = append(ds, Drafts(r, n)...)
		changes = append(changes, RuleChange{Rule: n, Disabled: disabled, Part: part})
	}
	return ds, changes
}
