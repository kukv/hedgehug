package saves

import (
	"slices"

	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/infra/gitstore/filemerge"
)

// droppedRules は、合わせ c の 2 つの親のそれぞれについて、c の purged.jsonl のプロジェクトのうち、c のワークスペースの
// ログにルールの行がなく、親のログにルールの行があるもの(決定 C5b1-6)。ルールの持ち主は、その版のログの scope で
// 決める。New と並びは droppedPlan と同じ。
func droppedRules(c *object.Commit, base func() (*object.Tree, error)) ([]port.DroppedRules, error) {
	ds, err := droppedBy(c, base, filemerge.PurgedProjects, byRuleOwner)
	if err != nil {
		return nil, err
	}
	var out []port.DroppedRules
	for _, d := range ds {
		out = append(out, port.DroppedRules{Workspace: d.Workspace, Project: d.Target, Log: d.Log, New: d.New})
	}
	return out, nil
}

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
