package projectrule

import (
	"maps"
	"slices"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
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

// TemplateImpact は、テンプレートの削除でルールが変わるプロジェクト(どちらもプロジェクト ID の順)。
type TemplateImpact struct {
	Projects  []project.ID // ルールが変わる
	Unlimited []project.ID // 使うテンプレートが 0 件になり、すべてのテンプレートを使えるようになる
}

// TemplateDeleteImpact は、テンプレート id の削除でルールが変わるプロジェクトを数える(B5a-17)。
// Unlimited は、Templates に id があり、id のほかに、いまもあるテンプレートの ID がないもの。
// 削除済みのテンプレートの残りは数えない(judge.go の絞り込みと同じ)。ワークスペースにほかのテンプレートが
// あるかは見ない。画面(core)も同じ数え方をする。
func TemplateDeleteImpact(s workspace.Settings, set Set, id change.ID) TemplateImpact {
	var out TemplateImpact
	for _, p := range slices.Sorted(maps.Keys(set)) {
		r := set[p]
		listed := slices.Contains(r.Templates, id)
		if !listed && !slices.Contains(slices.Collect(maps.Values(r.TemplateDefaults)), string(id)) {
			continue
		}
		out.Projects = append(out.Projects, p)
		if listed && !slices.ContainsFunc(r.Templates, func(t change.ID) bool {
			_, ok := s.Template(t)
			return t != id && ok
		}) {
			out.Unlimited = append(out.Unlimited, p)
		}
	}
	return out
}
