package filemerge

import (
	"bytes"
	"encoding/json"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/planning"
)

// PurgedPlanTargets は、purged.jsonl の entity が sprint・release の行の target(完全に削除したスプリント・
// リリースの内部 ID)。読めない行は飛ばす。
func PurgedPlanTargets(purged []byte) map[string]bool {
	out := map[string]bool{}
	for _, line := range bytes.Split(purged, []byte("\n")) {
		var p struct {
			Entity string `json:"entity"`
			Target string `json:"target"`
		}
		if json.Unmarshal(line, &p) != nil || p.Target == "" {
			continue
		}
		if p.Entity == "sprint" || p.Entity == "release" {
			out[p.Target] = true
		}
	}
	return out
}

// DropPurgedPlan は、ワークスペースのログ log から、planning.PlanTarget が targets にある行を除く。
// ほかの行はバイト列のまま残す。
func DropPurgedPlan(log []byte, targets map[string]bool) []byte {
	if len(targets) == 0 {
		return log
	}
	var out []byte
	for _, line := range bytes.SplitAfter(log, []byte("\n")) {
		var r change.Record
		if json.Unmarshal(line, &r) == nil {
			if id, ok := planning.PlanTarget(r); ok && targets[string(id)] {
				continue
			}
		}
		out = append(out, line...)
	}
	return out
}
