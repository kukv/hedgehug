package transfer

import (
	"encoding/json"
	"maps"
	"slices"

	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// assigneeKey は、テンプレートの値で担当者を持つ組み込みプロパティのキー。
const assigneeKey change.ID = "assignee"

// SettingsPeople は、ワークスペースの設定に出てくる人(重複あり・順は問わない)。移動先に足す人を決める
// People の extra に渡す。テンプレートの本体と子の担当者・ユーザー型の値、Step の担当者
// (StepParent・StepNone は除く)、ルールの担当者の値(Unassigned は除く)。
func SettingsPeople(s workspace.Settings, rules []automation.Rule) []string {
	out := templatePeople(s)
	for _, r := range rules {
		if isRulePerson(r) {
			out = append(out, r.Value)
		}
	}
	return out
}

// templatePeople は、テンプレートの本体・子・Step に出てくる人。
func templatePeople(s workspace.Settings) []string {
	var out []string
	users := userKeys(s)
	collect := func(values map[change.ID]json.RawMessage) {
		for _, k := range users {
			if login, ok := loginOf(values[k]); ok {
				out = append(out, login)
			}
		}
	}
	steps := func(ss []workspace.TemplateStep) {
		for _, st := range ss {
			if isStepPerson(st.Assignee) {
				out = append(out, st.Assignee)
			}
		}
	}
	for _, t := range s.Templates {
		collect(t.Values)
		steps(t.Steps)
		for _, c := range t.Children {
			collect(c.Values)
			steps(c.Steps)
		}
	}
	return out
}

// RemapPeople は、アイテムの人の値のうち、移動先のメンバーの綴りが変わるものを書き直す行。
// 担当者・ユーザー型の値・ウォッチの順。綴りが同じものは書かない。ほかの値の引き直しはしない。
// people が nil(同じ Store)なら行はない。
func RemapPeople(it item.Item, s workspace.Settings, people PeopleMap) []change.Draft {
	if people == nil {
		return nil
	}
	r := &remapper{it: it, from: s, to: s, people: people}
	r.assignee()
	for _, id := range userProperties(s) {
		raw := it.Fields[id]
		if login, ok := loginOf(raw); ok {
			if to := r.login(login); to != login {
				r.draft(change.NewSet(item.FieldKey(id), raw, to))
			}
		}
	}
	r.watchers()
	return r.res.Drafts
}

// RemapSettingsPeople は、テンプレートとルールの人の値を移動先のメンバーの綴りに書き直す行
// (workspace.TemplateDrafts・automation.Drafts の差分)。綴りが変わらないものは行を出さない。
// people が nil(同じ Store)なら行はない。
func RemapSettingsPeople(s workspace.Settings, rules []automation.Rule, people PeopleMap) []change.Draft {
	if people == nil {
		return nil
	}
	r := &remapper{people: people}
	users := userKeys(s)
	var ds []change.Draft
	for _, t := range s.Templates {
		ds = append(ds, workspace.TemplateDrafts(t, r.template(t, users))...)
	}
	for _, rule := range rules {
		next := rule
		if isRulePerson(rule) {
			next.Value = r.login(rule.Value)
		}
		ds = append(ds, automation.Drafts(rule, next)...)
	}
	return ds
}

// template は、t の人の値を移動先の綴りに置き換えたコピー。
func (r *remapper) template(t workspace.Template, users []change.ID) workspace.Template {
	t.Values = r.values(t.Values, users)
	t.Steps = r.steps(t.Steps)
	t.Children = slices.Clone(t.Children)
	for i, c := range t.Children {
		t.Children[i].Values = r.values(c.Values, users)
		t.Children[i].Steps = r.steps(c.Steps)
	}
	return t
}

func (r *remapper) values(v map[change.ID]json.RawMessage, users []change.ID) map[change.ID]json.RawMessage {
	out := maps.Clone(v)
	for _, k := range users {
		if login, ok := loginOf(v[k]); ok {
			if to := r.login(login); to != login {
				b, _ := json.Marshal(to)
				out[k] = b
			}
		}
	}
	return out
}

func (r *remapper) steps(ss []workspace.TemplateStep) []workspace.TemplateStep {
	out := slices.Clone(ss)
	for i, st := range out {
		if isStepPerson(st.Assignee) {
			out[i].Assignee = r.login(st.Assignee)
		}
	}
	return out
}

// userKeys は、テンプレートの値で人を持つキー。組み込みの担当者と、ユーザー型の追加プロパティの内部 ID。
func userKeys(s workspace.Settings) []change.ID {
	return append([]change.ID{assigneeKey}, userProperties(s)...)
}

// loginOf は、JSON の文字列の値。空・文字列でないものは含まない。
func loginOf(raw json.RawMessage) (string, bool) {
	var login string
	if json.Unmarshal(raw, &login) != nil || login == "" {
		return "", false
	}
	return login, true
}

func isStepPerson(a string) bool {
	return a != "" && a != workspace.StepParent && a != workspace.StepNone
}

func isRulePerson(r automation.Rule) bool {
	return r.Action == automation.ActionAssignee && r.Value != "" && r.Value != automation.Unassigned
}
