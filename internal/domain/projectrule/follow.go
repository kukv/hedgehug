package projectrule

import (
	"maps"
	"slices"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// StatusDeleteDrafts は、ステータス id の削除に、プロジェクト別のルールを追従させる行(B5a-16)。
// プロジェクト ID の順に、使わないステータス・ワークフローの制約(id の行の許可先と、ほかの行の id。
// workspace.WorkflowDeleteDrafts と同じ規則)の順で出す。
func StatusDeleteDrafts(set Set, id change.ID) []change.Draft {
	var ds []change.Draft
	for _, p := range slices.Sorted(maps.Keys(set)) {
		r := set[p]
		if slices.Contains(r.Hidden, id) {
			ds = append(ds, change.NewRemove(HiddenField(p), id))
		}
		for _, from := range slices.Sorted(maps.Keys(r.Workflow)) {
			switch {
			case from == id:
				for _, to := range r.Workflow[from] {
					ds = append(ds, change.NewRemove(WorkflowField(p, from), to))
				}
			case slices.Contains(r.Workflow[from], id):
				ds = append(ds, change.NewRemove(WorkflowField(p, from), id))
			}
		}
	}
	return ds
}

// RuleDeleteDrafts は、ワークスペースのルール rule の削除に、プロジェクトの外すルールを追従させる行。
// プロジェクト ID の順に、disabled_rules から rule を remove する。
func RuleDeleteDrafts(set Set, rule change.ID) []change.Draft {
	var ds []change.Draft
	for _, p := range slices.Sorted(maps.Keys(set)) {
		if slices.Contains(set[p].Disabled, rule) {
			ds = append(ds, change.NewRemove(DisabledField(p), rule))
		}
	}
	return ds
}

// TemplateDropDrafts は、テンプレート id の削除・種類の変更に、プロジェクト別のルールを追従させる行
// (B5a-16)。プロジェクト ID の順に、使うテンプレートの remove、id を指す種類ごとの既定(種類の順)を
// 設定なしに戻す set の順で出す。
func TemplateDropDrafts(set Set, id change.ID) []change.Draft {
	var ds []change.Draft
	for _, p := range slices.Sorted(maps.Keys(set)) {
		r := set[p]
		if slices.Contains(r.Templates, id) {
			ds = append(ds, change.NewRemove(TemplatesField(p), id))
		}
		for _, kind := range slices.Sorted(maps.Keys(r.TemplateDefaults)) {
			if r.TemplateDefaults[kind] == string(id) {
				ds = append(ds, change.NewSet(TemplateDefaultField(p, kind), string(id), nil))
			}
		}
	}
	return ds
}
