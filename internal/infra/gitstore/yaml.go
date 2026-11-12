package gitstore

import (
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
	Title     string `yaml:"title"`
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
		v = projectYAML{ID: string(p.ID), Workspace: string(p.Place.Workspace), Number: p.Place.Number, Title: p.Title}
	case change.KindItem:
		it, ok := item.FromLog(log)
		if !ok {
			return nil, nil
		}
		v = itemToYAML(it)
	}
	return encodeYAML(v)
}

func itemToYAML(it item.Item) itemYAML {
	y := itemYAML{
		ID: string(it.ID), Workspace: string(it.Place.Workspace), Project: string(it.Place.Project),
		Number: it.Place.Number, Kind: string(it.Kind), Rank: it.Rank, Title: it.Title,
		Created: createdYAML{At: it.Created.At.Format(change.TimeLayout), By: it.Created.By},
	}
	if it.Place.Parent != nil {
		p := string(*it.Place.Parent)
		y.Parent = &p
	}
	return y
}
