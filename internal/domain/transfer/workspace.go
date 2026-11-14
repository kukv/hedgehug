package transfer

import (
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// WorkspaceSet は、ワークスペースを移すときの、移すもの全体。Projects と Items は削除済みを含み、
// 引数の順のまま。Outside は、ワークスペースの外のアイテムとの関連(両方の側)。
type WorkspaceSet struct {
	Workspace workspace.ID
	Projects  []project.Project
	Items     []item.Item
	Outside   []Cut
}

// Counts は、確認ダイアログに出す件数(B5c-4)。
// Projects は削除されていないプロジェクト、Items は削除されていないプロジェクトの削除されていないアイテム、
// Deleted は残り(削除済みのプロジェクトと、自分が削除済みか削除済みのプロジェクトの下のアイテム)。
type Counts struct {
	Projects, Items, Deleted int
}

// CollectWorkspace は、ws のすべてのプロジェクトとアイテム(削除済みを含む)と、
// 集まりの外の(items にいる)アイテムとの関連を集める。
func CollectWorkspace(ws workspace.ID, projects []project.Project, items []item.Item) WorkspaceSet {
	s := WorkspaceSet{Workspace: ws}
	for _, p := range projects {
		if p.Place.Workspace == ws {
			s.Projects = append(s.Projects, p)
		}
	}
	inside := map[item.ID]bool{}
	for _, it := range items {
		if it.Place.Workspace == ws {
			s.Items = append(s.Items, it)
			inside[it.ID] = true
		}
	}
	s.Outside = cutsOutside(func(id item.ID) bool { return inside[id] }, items)
	return s
}

// Counts は、集まりの件数を数える。
func (s WorkspaceSet) Counts() Counts {
	live := map[project.ID]bool{}
	var c Counts
	for _, p := range s.Projects {
		if p.Deleted == nil {
			live[p.ID] = true
			c.Projects++
		} else {
			c.Deleted++
		}
	}
	for _, it := range s.Items {
		if it.Deleted == nil && live[it.Place.Project] {
			c.Items++
		} else {
			c.Deleted++
		}
	}
	return c
}
