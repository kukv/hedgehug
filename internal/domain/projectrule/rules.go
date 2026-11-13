// Package projectrule は、プロジェクト別のルール(使うステータス・ワークフローの制約・テンプレート・自動化。B5a)を持つ。
package projectrule

import (
	"encoding/json"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// None は、template_default の値で「テンプレートなし」を表す(B5a-2)。
const None = "@none"

const prefix = "project_rules/"

// Rules は、1 つのプロジェクトのルール。どれも空ならワークスペースの設定のまま。
type Rules struct {
	Hidden           []change.ID               // 使わないステータス
	Workflow         map[change.ID][]change.ID // From ごとの、変更できる To
	Templates        []change.ID               // 使うテンプレート
	TemplateDefaults map[string]string         // 種類ごとの既定(テンプレート ID か None)
	Disabled         []change.ID               // 外すワークスペースのルール
}

// Empty は、どのルールも持たないか。
func (r Rules) Empty() bool {
	return len(r.Hidden) == 0 && len(r.Workflow) == 0 && len(r.Templates) == 0 &&
		len(r.TemplateDefaults) == 0 && len(r.Disabled) == 0
}

// Set は、ルールを持つプロジェクトごとのルール(B5a-4)。空のルールのプロジェクトは入らない。
type Set map[project.ID]Rules

// Of は、p のルール。なければゼロ値。
func (s Set) Of(p project.ID) Rules { return s[p] }

func field(p project.ID, name string) string { return prefix + string(p) + "/" + name }

// HiddenField は、使わないステータスの項目名(add・remove)。
func HiddenField(p project.ID) string { return field(p, "hidden_statuses") }

// WorkflowField は、from から変更できる先の項目名(add・remove。値は To)。
func WorkflowField(p project.ID, from change.ID) string { return field(p, "workflow/"+string(from)) }

// TemplatesField は、使うテンプレートの項目名(add・remove)。
func TemplatesField(p project.ID) string { return field(p, "templates") }

// DisabledField は、外すルールの項目名(add・remove)。
func DisabledField(p project.ID) string { return field(p, "disabled_rules") }

// TemplateDefaultField は、種類 kind の既定テンプレートの項目名(set)。
func TemplateDefaultField(p project.ID, kind string) string {
	return field(p, "template_default/"+kind)
}

// IsMulti は、f が値を複数持つ項目(add・remove)か。workspace の Multi に同じ判定がある。
func IsMulti(f string) bool {
	_, rest, ok := split(f)
	switch {
	case !ok:
		return false
	case rest == "hidden_statuses", rest == "templates", rest == "disabled_rules":
		return true
	}
	from, ok := strings.CutPrefix(rest, "workflow/")
	return ok && from != "" && !strings.Contains(from, "/")
}

// split は、f を project_rules/<P>/<rest> に分ける。
func split(f string) (project.ID, string, bool) {
	body, ok := strings.CutPrefix(f, prefix)
	if !ok {
		return "", "", false
	}
	p, rest, ok := strings.Cut(body, "/")
	return project.ID(p), rest, ok && p != "" && rest != ""
}

// FromLog は、ワークスペースのログのプロジェクト別のルール。
func FromLog(log []change.Record) Set {
	st := change.Resolve(log, change.Options{Multi: IsMulti})
	rules := map[project.ID]*Rules{}
	of := func(p project.ID) *Rules {
		if rules[p] == nil {
			rules[p] = &Rules{}
		}
		return rules[p]
	}
	for f, vs := range st.Lists {
		if p, rest, ok := split(f); ok {
			addList(of(p), rest, ids(vs))
		}
	}
	for f, raw := range st.Values {
		p, rest, ok := split(f)
		kind, isDefault := strings.CutPrefix(rest, "template_default/")
		var v string
		if !ok || !isDefault || kind == "" || json.Unmarshal(raw, &v) != nil || v == "" {
			continue // null は「設定なし」
		}
		r := of(p)
		if r.TemplateDefaults == nil {
			r.TemplateDefaults = map[string]string{}
		}
		r.TemplateDefaults[kind] = v
	}
	out := Set{}
	for p, r := range rules {
		if !r.Empty() {
			out[p] = *r
		}
	}
	return out
}

func addList(r *Rules, rest string, vs []change.ID) {
	if len(vs) == 0 {
		return
	}
	switch rest {
	case "hidden_statuses":
		r.Hidden = vs
	case "templates":
		r.Templates = vs
	case "disabled_rules":
		r.Disabled = vs
	default:
		if from, ok := strings.CutPrefix(rest, "workflow/"); ok {
			if r.Workflow == nil {
				r.Workflow = map[change.ID][]change.ID{}
			}
			r.Workflow[change.ID(from)] = vs
		}
	}
}

func ids(raws []json.RawMessage) []change.ID {
	var out []change.ID
	for _, raw := range raws {
		var id change.ID
		if json.Unmarshal(raw, &id) == nil && id != "" {
			out = append(out, id)
		}
	}
	return out
}
