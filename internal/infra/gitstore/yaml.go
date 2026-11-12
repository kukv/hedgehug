package gitstore

import (
	"sort"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// 読む用の YAML の形。キーの並びは保存形式「アイテムの例」と同じ。行末の注釈(値の名前・表示の ID)は
// トラック A・B が足す。

type memberYAML struct {
	Login string `yaml:"login"`
	Name  string `yaml:"name"`
}

type workspaceYAML struct {
	ID     string `yaml:"id"`
	Name   string `yaml:"name"`
	Prefix string `yaml:"prefix"`
}

type projectYAML struct {
	ID        string `yaml:"id"`
	Workspace string `yaml:"workspace"`
	Number    int    `yaml:"number"`
	Rank      string `yaml:"rank"`
	Title     string `yaml:"title"`
	Owner     string `yaml:"owner"`
	Status    string `yaml:"status"`
	Start     string `yaml:"start"`
	End       string `yaml:"end"`
}

type createdYAML struct {
	At string `yaml:"at"`
	By string `yaml:"by"`
}

type itemYAML struct {
	ID        string      `yaml:"id"`
	Workspace string      `yaml:"workspace"`
	Project   string      `yaml:"project"`
	Parent    *string     `yaml:"parent"`
	Number    int         `yaml:"number"`
	Kind      string      `yaml:"kind"`
	Rank      string      `yaml:"rank"`
	Title     string      `yaml:"title"`
	Created   createdYAML `yaml:"created,flow"`
	Status    *string     `yaml:"status"`
	Priority  *string     `yaml:"priority"`
	Systems   []string    `yaml:"systems,flow"`
	Fields    yaml.Node   `yaml:"fields"`
	Watchers  []string    `yaml:"watchers,flow"`
}

// renderYAML は、エンティティのログから読む用の YAML を作る。作成の行がなければ nil。
func renderYAML(kind change.Kind, log []change.Record) ([]byte, error) {
	var v any
	switch kind {
	case change.KindMembers:
		ms := []memberYAML{}
		for _, m := range member.FromLog(log) {
			ms = append(ms, memberYAML(m))
		}
		v = ms
	case change.KindWorkspace:
		w, ok := workspace.FromLog(log)
		if !ok {
			return nil, nil
		}
		v = workspaceYAML{ID: string(w.ID), Name: w.Name, Prefix: w.Prefix}
	case change.KindProject:
		p, ok := project.FromLog(log)
		if !ok {
			return nil, nil
		}
		v = projectYAML{
			ID: string(p.ID), Workspace: string(p.Place.Workspace), Number: p.Place.Number, Rank: p.Rank,
			Title: p.Title, Owner: p.Owner, Status: string(p.Status), Start: p.Start, End: p.End,
		}
	case change.KindItem:
		it, ok := item.FromLog(log)
		if !ok {
			return nil, nil
		}
		v = itemToYAML(it, log)
	}
	return encodeYAML(v)
}

func itemToYAML(it item.Item, log []change.Record) itemYAML {
	y := itemYAML{
		ID: string(it.ID), Workspace: string(it.Place.Workspace), Project: string(it.Place.Project),
		Number: it.Place.Number, Kind: string(it.Kind), Rank: it.Rank, Title: it.Title,
		Created:  createdYAML{At: it.Created.At.Format(change.TimeLayout), By: it.Created.By},
		Status:   idPtr(it.Status),
		Priority: idPtr(it.Priority),
		Systems:  []string{},
		Fields:   fieldsNode(it, log),
		Watchers: it.Watchers,
	}
	for _, s := range it.Systems {
		y.Systems = append(y.Systems, string(s))
	}
	if it.Place.Parent != nil {
		p := string(*it.Place.Parent)
		y.Parent = &p
	}
	return y
}

// idPtr は、空の ID を null にする。
func idPtr(id change.ID) *string {
	if id == "" {
		return nil
	}
	s := string(id)
	return &s
}

// fieldsNode は、追加プロパティの値(プロパティの ID の辞書順)。複数選択は [..]、単一選択は 1 つの値。
func fieldsNode(it item.Item, log []change.Record) yaml.Node {
	n := yaml.Node{Kind: yaml.MappingNode}
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
		key := &yaml.Node{Kind: yaml.ScalarNode, Value: p}
		vs := it.Choices[change.ID(p)]
		val := &yaml.Node{Kind: yaml.ScalarNode, Value: string(vs[0])}
		if item.MultiChoice(log, change.ID(p)) {
			val = &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
			for _, v := range vs {
				val.Content = append(val.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: string(v)})
			}
		}
		n.Content = append(n.Content, key, val)
	}
	return n
}
