package repair

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
)

// RuleScopes は、専用ルールの内部 ID と、その rules/<R>/scope の最新の値(保存形式「同期の修復」の (9)。決定 C5b1-2)。
// 削除済みのルールも入る。scope のないルール(ワークスペース全体のルール)は入らない。
func RuleScopes(wsLog []change.Record) map[change.ID]project.ID {
	out := map[change.ID]project.ID{}
	for f, raw := range change.Resolve(wsLog, change.Options{}).Values {
		parts := strings.Split(f, "/")
		if len(parts) != 3 || parts[0] != "rules" || parts[2] != "scope" {
			continue
		}
		var p project.ID
		if json.Unmarshal(raw, &p) == nil && p != "" {
			out[change.ID(parts[1])] = p
		}
	}
	return out
}

// RuleOwner は、行 r のルールを持つプロジェクト。project_rules/<P>/… は P、rules/<R>/…(rules/<R> の削除を含む)は
// scopes にある R の scope。持ち主がなければ false(ワークスペース全体のルールの行や、ルールでない行)。
func RuleOwner(r change.Record, scopes map[change.ID]project.ID) (project.ID, bool) {
	if body, ok := strings.CutPrefix(r.F, "project_rules/"); ok {
		p, _, _ := strings.Cut(body, "/")
		return project.ID(p), p != ""
	}
	if body, ok := strings.CutPrefix(r.F, "rules/"); ok {
		id, _, _ := strings.Cut(body, "/")
		p, ok := scopes[change.ID(id)]
		return p, ok
	}
	return "", false
}

// OrphanRules は、ワークスペースにないプロジェクト Project のルールを消す行。
type OrphanRules struct {
	Project project.ID
	Drafts  []change.Draft
}

// Orphans は、持ち主のプロジェクトが inWorkspace でないルールを、プロジェクトの内部 ID の順で返す(決定 C5b1-3)。
// 行は、プロジェクト別のルールを 4 区画ぶん戻す行(使うステータス・ワークフロー・テンプレート・自動化の順)と、
// 消していない専用ルールの削除(内部 ID の順)。外すルールにあるワークスペース全体のルールは、外すだけで消さない。
// 行のないプロジェクトは入らない。
func Orphans(wsLog []change.Record, inWorkspace func(project.ID) bool) []OrphanRules {
	rules := projectrule.FromLog(wsLog)
	own := map[project.ID][]change.ID{}
	for _, r := range automation.FromLog(wsLog) {
		if r.Scope != "" {
			own[r.Scope] = append(own[r.Scope], r.ID)
		}
	}
	owners := map[project.ID]bool{}
	for p := range rules {
		owners[p] = true
	}
	for p := range own {
		owners[p] = true
	}
	sections := []projectrule.Section{
		projectrule.SectionStatuses, projectrule.SectionWorkflow,
		projectrule.SectionTemplates, projectrule.SectionAutomation,
	}
	var out []OrphanRules
	for _, p := range slices.Sorted(maps.Keys(owners)) {
		if inWorkspace(p) {
			continue
		}
		var ds []change.Draft
		for _, sec := range sections {
			ds = append(ds, projectrule.ResetDrafts(rules.Of(p), p, sec)...)
		}
		for _, id := range own[p] { // FromLog は内部 ID の順
			ds = append(ds, automation.DeleteDraft(id))
		}
		if len(ds) > 0 {
			out = append(out, OrphanRules{Project: p, Drafts: ds})
		}
	}
	return out
}
