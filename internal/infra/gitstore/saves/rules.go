package saves

import (
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/repair"
)

// byRuleOwner は、rows のルールの行を、持ち主のプロジェクトが targets にあるものごとに分けたもの。専用ルールの
// 持ち主は、log の scope で決める。
func byRuleOwner(log, rows []change.Record, targets map[string]bool) map[change.ID][]change.Record {
	scopes := repair.RuleScopes(log)
	out := map[change.ID][]change.Record{}
	for _, r := range rows {
		if p, ok := repair.RuleOwner(r, scopes); ok && targets[string(p)] {
			out[p] = append(out[p], r)
		}
	}
	return out
}

// hasNewRules は、ds に、共通の祖先にない行を持つものがあるか。
func hasNewRules(ds []port.DroppedRules) bool {
	return slices.ContainsFunc(ds, func(d port.DroppedRules) bool { return len(d.New) > 0 })
}
