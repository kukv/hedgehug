package gitstore

import (
	"sort"
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// itemToYAML は、item.yaml の中身。status・priority・systems・fields の行末に、値の名前を注釈で書く(B2-19)。
// 名前が分からない値(消した値、設定のログがないワークスペース)には注釈を付けない。
func itemToYAML(it item.Item, log []change.Record, settings settingsOf) itemYAML {
	var st workspace.Settings
	if settings != nil {
		st, _ = settings(it.Place.Workspace)
	}
	y := itemYAML{
		ID: string(it.ID), Workspace: string(it.Place.Workspace), Project: string(it.Place.Project),
		Number: it.Place.Number, Kind: string(it.Kind), Rank: it.Rank, Title: it.Title,
		Created:  createdYAML{At: it.Created.At.Format(change.TimeLayout), By: it.Created.By},
		Status:   idNode(it.Status),
		Priority: idNode(it.Priority),
		Systems:  systemsNode(it.Systems, st),
		Fields:   fieldsNode(it, log, st),
		Watchers: it.Watchers,
	}
	y.Status.LineComment = comment(valueName(st.Statuses, it.Status))
	y.Priority.LineComment = comment(valueName(st.Priorities, it.Priority))
	if it.Place.Parent != nil {
		p := string(*it.Place.Parent)
		y.Parent = &p
	}
	return y
}

func systemsNode(ids []change.ID, st workspace.Settings) *yaml.Node {
	vs := make([]string, len(ids))
	for i, id := range ids {
		vs[i] = string(id)
	}
	n := flowSeq(vs...)
	n.LineComment = comment(strings.Join(namesOf(st.Systems, ids), ", "))
	return n
}

// namesOf は、ids のうち一覧にあるものの名前(ids の並び)。
func namesOf(vs []workspace.Value, ids []change.ID) []string {
	var out []string
	for _, id := range ids {
		if name := valueName(vs, id); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// fieldsNode は、追加プロパティの値(プロパティの ID の辞書順)。複数選択は [..]、単一選択は 1 つの値。
// 行末の注釈は「プロパティ名: 値の名前」。
func fieldsNode(it item.Item, log []change.Record, st workspace.Settings) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode}
	if len(it.Choices) == 0 {
		n.Style = yaml.FlowStyle
		return n
	}
	props := make([]string, 0, len(it.Choices))
	for p := range it.Choices {
		props = append(props, string(p))
	}
	sort.Strings(props)
	for _, p := range props {
		vs := it.Choices[change.ID(p)]
		val := strNode(string(vs[0]))
		if item.MultiChoice(log, change.ID(p)) {
			ids := make([]string, len(vs))
			for i, v := range vs {
				ids[i] = string(v)
			}
			val = flowSeq(ids...)
		}
		val.LineComment = fieldComment(st, change.ID(p), vs)
		n.Content = append(n.Content, key(p), val)
	}
	return n
}

// fieldComment は「プロパティ名: 値の名前, …」。プロパティか値の名前が分からなければ空。
func fieldComment(st workspace.Settings, p change.ID, vs []change.ID) string {
	prop, ok := st.Property(p)
	if !ok {
		return ""
	}
	names := namesOf(prop.Choices, vs)
	if len(names) == 0 {
		return ""
	}
	return comment(prop.Name + ": " + strings.Join(names, ", "))
}
