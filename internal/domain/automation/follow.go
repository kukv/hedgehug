package automation

import (
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// ステータスの削除で書き換えた側(RuleChange.Part。通知の文にそのまま入る)。
const (
	PartTrigger = "きっかけ"
	PartAction  = "すること"
)

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
			n.TriggerStatus, part = repl, PartTrigger
		}
		if n.Action == ActionStatus && n.Value == string(id) {
			n.Value, part = string(repl), PartAction
		}
		if part == "" {
			continue
		}
		disabled := CheckSameStatus(n) != nil && r.Enabled // もともと有効で、今回無効にしたときだけ
		if CheckSameStatus(n) != nil {
			n.Enabled = false
		}
		ds = append(ds, Drafts(r, n)...)
		changes = append(changes, RuleChange{Rule: n, Disabled: disabled, Part: part})
	}
	return ds, changes
}

// FollowedFromRows は、1 つの保存がワークスペースのログに足した行 rows から、削除したステータスの ID と、
// それに合わせて書き換わったルール(通知の材料)を読む。rules は書き換えたあとのルール。
// ステータスの削除の行がなければ、削除したステータスは空。
func FollowedFromRows(rows []change.Record, rules []Rule) (change.ID, []RuleChange) {
	var deleted change.ID
	fields := map[change.ID]map[string]bool{}
	for _, r := range rows {
		if id, ok := strings.CutPrefix(r.F, "statuses/"); ok && r.T == change.Delete && !strings.Contains(id, "/") {
			deleted = change.ID(id)
			continue
		}
		parts := strings.Split(r.F, "/")
		if len(parts) != 3 || parts[0]+"/" != prefix || r.T != change.Set {
			continue
		}
		id := change.ID(parts[1])
		if fields[id] == nil {
			fields[id] = map[string]bool{}
		}
		fields[id][parts[2]] = true
	}
	if deleted == "" {
		return "", nil
	}
	var changes []RuleChange
	for _, rule := range rules {
		f := fields[rule.ID]
		part := ""
		switch {
		case f["value"]:
			part = PartAction
		case f["trigger_status"]:
			part = PartTrigger
		}
		if part != "" {
			changes = append(changes, RuleChange{Rule: rule, Disabled: f["enabled"], Part: part})
		}
	}
	return deleted, changes
}
