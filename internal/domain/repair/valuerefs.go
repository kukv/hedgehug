package repair

import (
	"maps"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// RefKind は、削除された値を指すものの種類(決定 C5b2-6)。
type RefKind int

const (
	RefProject         RefKind = iota // プロジェクト別のルール
	RefTemplateDefault                // ワークスペースの種類ごとの既定のテンプレート
	RefRule                           // 自動化のルール
)

// RefRepair は、削除された値 Value を指すもの 1 つを直す行(1 回の保存)。Project は RefProject のときだけ、
// TemplateKind は RefTemplateDefault のときだけ、Rule は RefRule のときだけ持つ。RefRule の Value は、ルールが指す
// 削除された値のうち automation.Statuses の順で最初のもの。
type RefRepair struct {
	Kind         RefKind
	Project      project.ID
	Value        change.ID
	TemplateKind string
	Rule         change.ID
	Drafts       []change.Draft
}

// DanglingRefs は、取り消されていない削除のある値(statuses/<ID>・templates/<ID>)を指すものを直す行(決定 C5b2-6)。
// プロジェクト別のルール(プロジェクトの内部 ID の順、その中は値の ID の順)、ワークスペースの種類ごとの既定
// (workspace.Kinds の順)、有効なルール(内部 ID の順)の順に返す。行のないものは入れない。
// ワークスペースのワークフローは直さない(決定 C5b2-7)。
func DanglingRefs(log []change.Record) []RefRepair {
	deleted := deletedValues(log, "statuses", "templates")
	if len(deleted) == 0 {
		return nil
	}
	out := projectRefs(log, deleted)
	out = append(out, defaultRefs(log, deleted)...)
	return append(out, ruleRefs(log, deleted)...)
}

// projectRefs は、プロジェクト別のルールの分。行は、そのプロジェクトだけの Set に、値の削除の追従と同じ規則を当てたもの。
// 同じプロジェクトで先の値の修復が持つ行は、あとの値の修復から除き、行がなくなった修復は入れない。
func projectRefs(log []change.Record, deleted map[string][]change.Record) []RefRepair {
	ids := map[change.ID]bool{} // 値の ID → テンプレートか
	for f := range deleted {
		list, id, _ := strings.Cut(f, "/")
		ids[change.ID(id)] = list == "templates"
	}
	values := slices.Sorted(maps.Keys(ids))
	set := projectrule.FromLog(log)
	var out []RefRepair
	for _, p := range slices.Sorted(maps.Keys(set)) {
		one := projectrule.Set{p: set[p]}
		seen := map[string]bool{} // このプロジェクトで出した行。辺の両端が削除されたときの同じ行を、先の値の修復だけに入れる
		for _, id := range values {
			ds := projectrule.StatusDeleteDrafts(one, id)
			if ids[id] {
				ds = projectrule.TemplateDropDrafts(one, id)
			}
			ds = slices.DeleteFunc(ds, func(d change.Draft) bool {
				k := string(d.T) + "\x00" + d.F + "\x00" + string(d.From) + "\x00" + string(d.To)
				dup := seen[k]
				seen[k] = true
				return dup
			})
			if len(ds) > 0 {
				out = append(out, RefRepair{Kind: RefProject, Project: p, Value: id, Drafts: ds})
			}
		}
	}
	return out
}

// defaultRefs は、ワークスペースの種類ごとの既定の分。既定は削除済みのテンプレートを隠すので、削除がないものとして読む。
func defaultRefs(log []change.Record, deleted map[string][]change.Record) []RefRepair {
	var fs []string
	for f := range deleted {
		if strings.HasPrefix(f, "templates/") {
			fs = append(fs, f)
		}
	}
	if len(fs) == 0 {
		return nil
	}
	st, _ := workspace.SettingsReviving(log, fs...)
	var out []RefRepair
	for _, k := range workspace.Kinds {
		id, ok := st.TemplateDefaults[k]
		if ok && len(deleted[workspace.TemplateField(id)]) > 0 {
			out = append(out, RefRepair{Kind: RefTemplateDefault, Value: id, TemplateKind: k, Drafts: []change.Draft{
				change.NewSet("template_default/"+k, id, nil),
			}})
		}
	}
	return out
}

// ruleRefs は、削除されたステータスを指す有効なルールを無効にする分。無効のルールは書かない。
func ruleRefs(log []change.Record, deleted map[string][]change.Record) []RefRepair {
	var out []RefRepair
	for _, r := range automation.FromLog(log) {
		if !r.Enabled {
			continue
		}
		i := slices.IndexFunc(automation.Statuses(r), func(id change.ID) bool { return len(deleted["statuses/"+string(id)]) > 0 })
		if i < 0 {
			continue
		}
		off := r
		off.Enabled = false
		out = append(out, RefRepair{Kind: RefRule, Value: automation.Statuses(r)[i], Rule: r.ID, Drafts: automation.Drafts(r, off)})
	}
	return out
}
