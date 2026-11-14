package filemerge

import (
	"bytes"
	"encoding/json"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/repair"
)

// PurgedProjects は、purged.jsonl の entity が project の行の target(完全に削除したプロジェクトの内部 ID)。
// 読めない行は PurgedPlanTargets と同じく飛ばす。
func PurgedProjects(purged []byte) map[string]bool {
	return purgedTargets(purged, "project")
}

// DropPurgedRules は、ワークスペースのログ log から、持ち主のプロジェクトが projects にあるルールの行
// (project_rules/<P>/… と、scope が P の専用ルールの行)を除く。専用ルールの scope は、除く前の log から読む。
// ほかの行はバイト列のまま残す。
func DropPurgedRules(log []byte, projects map[string]bool) []byte {
	if len(projects) == 0 {
		return log
	}
	var recs []change.Record
	for _, line := range bytes.Split(log, []byte("\n")) {
		var r change.Record
		if json.Unmarshal(line, &r) == nil {
			recs = append(recs, r)
		}
	}
	scopes := repair.RuleScopes(recs)
	var out []byte
	for _, line := range bytes.SplitAfter(log, []byte("\n")) {
		var r change.Record
		if json.Unmarshal(line, &r) == nil {
			if p, ok := repair.RuleOwner(r, scopes); ok && projects[string(p)] {
				continue
			}
		}
		out = append(out, line...)
	}
	return out
}
