package gitstore

import (
	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// settingsOf は、ワークスペースの設定を引く。ログがなければ false(そのアイテムの注釈は付けない)。
type settingsOf func(workspace.ID) (workspace.Settings, bool)

func strNode(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
}

func boolNode(b bool) *yaml.Node {
	v := "false"
	if b {
		v = "true"
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: v}
}

// idNode は、ID の値。空なら null。
func idNode(id change.ID) *yaml.Node {
	if id == "" {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}
	}
	return strNode(string(id))
}

func flowSeq(vs ...string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
	for _, v := range vs {
		n.Content = append(n.Content, strNode(v))
	}
	return n
}

// mapping は、キーと値を並べた辞書。flow なら 1 行になる。
func mapping(flow bool, kv ...*yaml.Node) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode, Content: kv}
	if flow {
		n.Style = yaml.FlowStyle
	}
	return n
}

func key(k string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k} }

// workspaceToYAML は、workspace.yaml の中身(保存形式「ファイル構成」。キーの並びはこの順)。
func workspaceToYAML(w workspace.Workspace, rank string, s workspace.Settings) *yaml.Node {
	statusName := valueName(s.Statuses, s.InitialStatus)
	prioName := valueName(s.Priorities, s.InitialPriority)
	initStatus, initPrio := idNode(s.InitialStatus), idNode(s.InitialPriority)
	initStatus.LineComment, initPrio.LineComment = comment(statusName), comment(prioName)
	return mapping(false,
		key("id"), strNode(string(w.ID)),
		key("name"), strNode(w.Name),
		key("prefix"), strNode(w.Prefix),
		key("rank"), strNode(rank),
		key("properties"), propertiesNode(s.Properties),
		key("statuses"), valuesNode(s.Statuses, true),
		key("priorities"), valuesNode(s.Priorities, false),
		key("systems"), valuesNode(s.Systems, false),
		key("initial_status"), initStatus,
		key("initial_priority"), initPrio,
		key("category_defaults"), categoryDefaultsNode(s),
		key("workflow"), workflowNode(s),
		key("templates"), templatesNode(s.Templates),
		key("template_defaults"), templateDefaultsNode(s),
	)
}

// comment は、行末の注釈。text が空なら注釈なし。
func comment(text string) string {
	if text == "" {
		return ""
	}
	return "# " + text
}

// valueName は、一覧にある値の名前。なければ空(注釈を付けない)。
func valueName(vs []workspace.Value, id change.ID) string {
	for _, v := range vs {
		if v.ID == id {
			return v.Name
		}
	}
	return ""
}

func seqOf(items []*yaml.Node) *yaml.Node {
	n := &yaml.Node{Kind: yaml.SequenceNode, Content: items}
	if len(items) == 0 {
		n.Style = yaml.FlowStyle
	}
	return n
}

func propertiesNode(ps []workspace.Property) *yaml.Node {
	items := make([]*yaml.Node, 0, len(ps))
	for _, p := range ps {
		kinds := make([]string, len(p.Kinds))
		copy(kinds, p.Kinds)
		kv := []*yaml.Node{
			key("id"), strNode(string(p.ID)),
			key("key"), strNode(p.Key),
			key("name"), strNode(p.Name),
			key("type"), strNode(string(p.Type)),
			key("builtin"), boolNode(p.Builtin),
			key("kinds"), flowSeq(kinds...),
			key("hidden"), boolNode(p.Hidden),
			key("rank"), strNode(p.Rank),
		}
		if len(p.Choices) > 0 {
			kv = append(kv, key("choices"), valuesNode(p.Choices, false))
		}
		items = append(items, mapping(len(p.Choices) == 0, kv...))
	}
	return seqOf(items)
}

// valuesNode は、値の一覧。分類(category)はステータスだけ。
func valuesNode(vs []workspace.Value, withCategory bool) *yaml.Node {
	items := make([]*yaml.Node, 0, len(vs))
	for _, v := range vs {
		kv := []*yaml.Node{
			key("id"), strNode(string(v.ID)),
			key("name"), strNode(v.Name),
			key("color"), strNode(string(v.Color)),
		}
		if withCategory {
			kv = append(kv, key("category"), strNode(string(v.Category)))
		}
		kv = append(kv, key("rank"), strNode(v.Rank))
		items = append(items, mapping(true, kv...))
	}
	return seqOf(items)
}

// categoryDefaultsNode は、分類の既定(分類の並び)。
func categoryDefaultsNode(s workspace.Settings) *yaml.Node {
	var kv []*yaml.Node
	for _, c := range workspace.Categories {
		if id, ok := s.CategoryDefaults[c]; ok {
			kv = append(kv, key(string(c)), strNode(string(id)))
		}
	}
	return mapping(true, kv...)
}
