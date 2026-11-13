package gitstore

import (
	"encoding/json"
	"slices"
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// workflowNode は、遷移の制約(ステータスの並び)。制限のない行は書かない。行末に名前の注釈。
func workflowNode(s workspace.Settings) *yaml.Node {
	var kv []*yaml.Node
	for _, v := range s.Statuses {
		to, restricted := s.Allowed(v.ID)
		if !restricted {
			continue
		}
		ids, names := make([]string, 0, len(to)), make([]string, 0, len(to))
		for _, id := range to {
			ids = append(ids, string(id))
			names = append(names, valueName(s.Statuses, id))
		}
		seq := flowSeq(ids...)
		seq.LineComment = comment(v.Name + " → " + strings.Join(names, "・")) // initial_status と同じく値の節に注釈
		kv = append(kv, key(string(v.ID)), seq)
	}
	return mapping(len(kv) == 0, kv...)
}

func templatesNode(ts []workspace.Template) *yaml.Node {
	items := make([]*yaml.Node, 0, len(ts))
	for _, t := range ts {
		children := make([]*yaml.Node, 0, len(t.Children))
		for _, c := range t.Children {
			children = append(children, mapping(true,
				key("id"), strNode(string(c.ID)), key("kind"), strNode(c.Kind), key("title"), strNode(c.Title),
				key("description"), strNode(c.Description), key("rank"), strNode(c.Rank),
				key("values"), valuesMap(c.Values), key("steps"), stepsNode(c.Steps)))
		}
		items = append(items, mapping(false,
			key("id"), strNode(string(t.ID)), key("name"), strNode(t.Name), key("kind"), strNode(t.Kind),
			key("title"), strNode(t.Title), key("description"), strNode(t.Description),
			key("values"), valuesMap(t.Values), key("children"), seqOf(children), key("steps"), stepsNode(t.Steps)))
	}
	return seqOf(items)
}

func stepsNode(ss []workspace.TemplateStep) *yaml.Node {
	items := make([]*yaml.Node, 0, len(ss))
	for _, s := range ss {
		items = append(items, mapping(true,
			key("id"), strNode(string(s.ID)), key("title"), strNode(s.Title),
			key("assignee"), strNode(s.Assignee), key("rank"), strNode(s.Rank)))
	}
	n := seqOf(items)
	n.Style = yaml.FlowStyle
	return n
}

// valuesMap は、テンプレートの値(キーの順、1 行)。値は JSON のまま YAML の値にする。
func valuesMap(vs map[change.ID]json.RawMessage) *yaml.Node {
	keys := make([]change.ID, 0, len(vs))
	for k := range vs {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	kv := make([]*yaml.Node, 0, 2*len(keys))
	for _, k := range keys {
		var v yaml.Node
		if yaml.Unmarshal(vs[k], &v) != nil || len(v.Content) == 0 {
			continue
		}
		flowAll(v.Content[0])
		kv = append(kv, key(string(k)), v.Content[0])
	}
	return mapping(true, kv...)
}

// flowAll は、取り出した節の辞書と並びを 1 行にする。
func flowAll(n *yaml.Node) {
	if n.Kind == yaml.MappingNode || n.Kind == yaml.SequenceNode {
		n.Style |= yaml.FlowStyle
	}
	for _, c := range n.Content {
		flowAll(c)
	}
}

// templateDefaultsNode は、種類ごとの既定(種類の並び)。
func templateDefaultsNode(s workspace.Settings) *yaml.Node {
	var kv []*yaml.Node
	for _, k := range workspace.Kinds {
		if id, ok := s.TemplateDefaults[k]; ok {
			kv = append(kv, key(k), strNode(string(id)))
		}
	}
	return mapping(true, kv...)
}

// rulesNode は、自動化のルール(作った順)。ステータスの値は行末に名前の注釈。専用ルールには scope を書く。
func rulesNode(s workspace.Settings, rules []automation.Rule, prefix string, projects projectOf) *yaml.Node {
	items := make([]*yaml.Node, 0, len(rules))
	for _, r := range rules {
		ts := idNode(r.TriggerStatus)
		ts.LineComment = comment(valueName(s.Statuses, r.TriggerStatus))
		v := strNode(r.Value)
		if r.Action == automation.ActionStatus {
			v.LineComment = comment(valueName(s.Statuses, change.ID(r.Value)))
		}
		kv := []*yaml.Node{
			key("id"), strNode(string(r.ID)),
			key("name"), strNode(r.Name),
			key("trigger"), strNode(string(r.Trigger)),
			key("trigger_status"), ts,
			key("all_merged"), boolNode(r.AllMerged),
			key("action"), strNode(string(r.Action)),
			key("value"), v,
			key("enabled"), boolNode(r.Enabled),
		}
		if r.Scope != "" {
			scope := strNode(string(r.Scope))
			scope.LineComment = comment(projectLabel(prefix, projects, r.Scope))
			kv = append(kv, key("scope"), scope)
		}
		items = append(items, mapping(false, kv...))
	}
	return seqOf(items)
}

// projectLabel は、プロジェクトの画面の ID(HH-P1)。ログがなければ空(注釈を付けない)。
func projectLabel(prefix string, projects projectOf, id project.ID) string {
	if p, ok := projects(id); ok {
		return p.DisplayID(prefix)
	}
	return ""
}

// namesComment は、ID の並びに付ける名前の注釈(「・」でつなぐ)。名前が 1 つも引けなければ空。
func namesComment(ids []change.ID, name func(change.ID) string) string {
	names, found := make([]string, len(ids)), false
	for i, id := range ids {
		names[i] = name(id)
		found = found || names[i] != ""
	}
	if !found {
		return ""
	}
	return comment(strings.Join(names, "・"))
}

func idsSeq(ids []change.ID, name func(change.ID) string) *yaml.Node {
	vs := make([]string, len(ids))
	for i, id := range ids {
		vs[i] = string(id)
	}
	n := flowSeq(vs...)
	n.LineComment = namesComment(ids, name)
	return n
}

// projectRulesNode は、プロジェクト別のルール(プロジェクトの内部 ID の順)。空のものは書かない。
// 値は ID で、行末に名前の注釈を付ける。
func projectRulesNode(s workspace.Settings, rules []automation.Rule, set projectrule.Set, prefix string, projects projectOf) *yaml.Node {
	pids := make([]project.ID, 0, len(set))
	for p := range set {
		pids = append(pids, p)
	}
	slices.Sort(pids)
	var kv []*yaml.Node
	for _, p := range pids {
		k := key(string(p))
		k.LineComment = comment(projectLabel(prefix, projects, p))
		kv = append(kv, k, projectBodyNode(s, rules, set[p]))
	}
	return mapping(false, kv...)
}

// projectBodyNode は、1 つのプロジェクトのルール。空の項目は書かない。
func projectBodyNode(s workspace.Settings, rules []automation.Rule, r projectrule.Rules) *yaml.Node {
	statusName := func(id change.ID) string { return valueName(s.Statuses, id) }
	templateName := func(id change.ID) string {
		i := slices.IndexFunc(s.Templates, func(t workspace.Template) bool { return t.ID == id })
		if i < 0 {
			return ""
		}
		return s.Templates[i].Name
	}
	ruleName := func(id change.ID) string {
		i := slices.IndexFunc(rules, func(x automation.Rule) bool { return x.ID == id })
		if i < 0 {
			return ""
		}
		return rules[i].Name
	}
	var body []*yaml.Node
	if len(r.Hidden) > 0 {
		body = append(body, key("hidden_statuses"), idsSeq(r.Hidden, statusName))
	}
	if len(r.Workflow) > 0 {
		body = append(body, key("workflow"), projectWorkflowNode(s, r.Workflow))
	}
	if len(r.Templates) > 0 {
		body = append(body, key("templates"), idsSeq(r.Templates, templateName))
	}
	if len(r.TemplateDefaults) > 0 {
		body = append(body, key("template_defaults"), projectDefaultsNode(r.TemplateDefaults, templateName))
	}
	if len(r.Disabled) > 0 {
		body = append(body, key("disabled_rules"), idsSeq(r.Disabled, ruleName))
	}
	return mapping(false, body...)
}

// projectWorkflowNode は、From ごとの、変更できる To(ステータスの並び、そのあとにない ID の並び)。
func projectWorkflowNode(s workspace.Settings, wf map[change.ID][]change.ID) *yaml.Node {
	name := func(id change.ID) string { return valueName(s.Statuses, id) }
	froms := make([]change.ID, 0, len(wf))
	for _, v := range s.Statuses {
		if _, ok := wf[v.ID]; ok {
			froms = append(froms, v.ID)
		}
	}
	var rest []change.ID
	for f := range wf {
		if !slices.Contains(froms, f) {
			rest = append(rest, f)
		}
	}
	slices.Sort(rest)
	var kv []*yaml.Node
	for _, from := range append(froms, rest...) {
		seq := idsSeq(wf[from], name)
		if c := namesComment(wf[from], name); c != "" {
			seq.LineComment = comment(name(from) + " → " + strings.TrimPrefix(c, "# "))
		}
		kv = append(kv, key(string(from)), seq)
	}
	return mapping(false, kv...)
}

// projectDefaultsNode は、種類ごとの既定(種類の並び)。「なし」は @none に「なし」の注釈。
func projectDefaultsNode(d map[string]string, templateName func(change.ID) string) *yaml.Node {
	var kv []*yaml.Node
	for _, k := range workspace.Kinds {
		v, ok := d[k]
		if !ok {
			continue
		}
		n := strNode(v)
		if v == projectrule.None {
			n.LineComment = comment("なし")
		} else {
			n.LineComment = comment(templateName(change.ID(v)))
		}
		kv = append(kv, key(k), n)
	}
	return mapping(false, kv...)
}
