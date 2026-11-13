package gitstore

import (
	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// 読む用の YAML の形。キーの並びは保存形式「アイテムの例」と同じ。行末の注釈(値の名前・表示の ID)は、
// 種類ごとの yaml_*.go が付ける。

type memberYAML struct {
	Login string `yaml:"login"`
	Name  string `yaml:"name"`
}

type projectYAML struct {
	ID        string       `yaml:"id"`
	Workspace string       `yaml:"workspace"`
	Number    int          `yaml:"number"`
	Rank      string       `yaml:"rank"`
	Title     string       `yaml:"title"`
	Owner     string       `yaml:"owner"`
	Status    string       `yaml:"status"`
	Start     string       `yaml:"start"`
	End       string       `yaml:"end"`
	Repos     []string     `yaml:"repos,flow"`
	Deleted   *deletedYAML `yaml:"deleted,flow"`
}

// deletedYAML は、削除済みの印(読む用の写し。正本は delete の行)。削除されていなければ書かず、null になる(A4b-2)。
type deletedYAML struct {
	Op   string `yaml:"op"`
	At   string `yaml:"at"`
	Root bool   `yaml:"root"`
}

func deletedToYAML(d *change.Deletion) *deletedYAML {
	if d == nil {
		return nil
	}
	return &deletedYAML{Op: string(d.Op), At: d.At.Format(change.TimeLayout), Root: d.Root}
}

type createdYAML struct {
	At string `yaml:"at"`
	By string `yaml:"by"`
}

type itemYAML struct {
	ID        string       `yaml:"id"`
	Workspace string       `yaml:"workspace"`
	Project   string       `yaml:"project"`
	Parent    *string      `yaml:"parent"`
	Number    int          `yaml:"number"`
	Kind      string       `yaml:"kind"`
	Rank      string       `yaml:"rank"`
	Title     string       `yaml:"title"`
	Created   createdYAML  `yaml:"created,flow"`
	Status    *yaml.Node   `yaml:"status"`
	Assignee  *string      `yaml:"assignee"`
	Priority  *yaml.Node   `yaml:"priority"`
	Start     *string      `yaml:"start"`
	End       *string      `yaml:"end"`
	Points    *float64     `yaml:"points"`
	Systems   *yaml.Node   `yaml:"systems"`
	Sprint    *yaml.Node   `yaml:"sprint"`
	Release   *yaml.Node   `yaml:"release"`
	Fields    *yaml.Node   `yaml:"fields"`
	Relations *yaml.Node   `yaml:"relations"`
	Estimate  *float64     `yaml:"estimate"`
	Links     *yaml.Node   `yaml:"links"`
	LinksOff  *yaml.Node   `yaml:"links_off"`
	Watchers  []string     `yaml:"watchers,flow"`
	Comments  *yaml.Node   `yaml:"comments"`
	Worklogs  *yaml.Node   `yaml:"worklogs"`
	Deleted   *deletedYAML `yaml:"deleted,flow"`
}

// renderYAML は、エンティティのログから読む用の YAML を作る。作成の行がなければ nil。
func renderYAML(kind change.Kind, log []change.Record, settings settingsOf, projects projectOf) ([]byte, error) {
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
		st, _ := workspace.SettingsFromLog(log)
		v = workspaceToYAML(w, workspace.RankOf(log), st, automation.FromLog(log), projectrule.FromLog(log), projects, planning.FromLog(log))
	case change.KindProject:
		p, ok := project.FromLog(log)
		if !ok {
			return nil, nil
		}
		v = projectYAML{
			ID: string(p.ID), Workspace: string(p.Place.Workspace), Number: p.Place.Number, Rank: p.Rank,
			Title: p.Title, Owner: p.Owner, Status: string(p.Status), Start: p.Start, End: p.End, Repos: p.Repos,
			Deleted: deletedToYAML(p.Deleted),
		}
	case change.KindItem:
		it, ok := item.FromLog(log)
		if !ok {
			return nil, nil
		}
		v = itemToYAML(it, log, settings)
	}
	return encodeYAML(v)
}
