package gitstore

import (
	"encoding/json"
	"slices"
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
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

// rulesNode は、自動化のルール(作った順)。ステータスの値は行末に名前の注釈。
func rulesNode(s workspace.Settings, rules []automation.Rule) *yaml.Node {
	items := make([]*yaml.Node, 0, len(rules))
	for _, r := range rules {
		ts := idNode(r.TriggerStatus)
		ts.LineComment = comment(valueName(s.Statuses, r.TriggerStatus))
		v := strNode(r.Value)
		if r.Action == automation.ActionStatus {
			v.LineComment = comment(valueName(s.Statuses, change.ID(r.Value)))
		}
		items = append(items, mapping(false,
			key("id"), strNode(string(r.ID)),
			key("name"), strNode(r.Name),
			key("trigger"), strNode(string(r.Trigger)),
			key("trigger_status"), ts,
			key("all_merged"), boolNode(r.AllMerged),
			key("action"), strNode(string(r.Action)),
			key("value"), v,
			key("enabled"), boolNode(r.Enabled),
		))
	}
	return seqOf(items)
}
