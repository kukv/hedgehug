package items

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// MoveCandidate は、「移動…」の移し先の候補。Epic の候補はプロジェクト(Project)、ほかは親になるアイテム(Item)で、
// どちらか一方だけが入る。ProjectID は、アイテムの候補なら属するプロジェクト、プロジェクトの候補なら自分の ID。
// Display は画面の ID(HH-12・HH-P2)。ProjectTitle は、アイテムの候補が属するプロジェクトの名前(プロジェクトの候補では空)。
type MoveCandidate struct {
	Item         *item.Item
	Project      *project.Project
	ProjectID    project.ID
	ProjectTitle string
	Display      string
}

// MoveCandidates は、id の移し先の候補を返す(02「同じワークスペースの中での移動」、A4b-12)。絞り込みは画面が行う。
// Epic にはほかのプロジェクト、Step には Story・Task・Bug、ほかには Epic を、ワークスペース全体から、削除されていないものだけ返す。
// 自分と下のもの、いまの親は除く。並びはプロジェクトの並び → ツリーの並び。ないか削除済みなら port.ErrNotFound。
func (s Service) MoveCandidates(ctx context.Context, store port.StoreID, id item.ID) ([]MoveCandidate, error) {
	var out []MoveCandidate
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		it, err := port.LiveItem(r, id)
		if err != nil {
			return err
		}
		w, err := r.Workspaces().Get(it.Place.Workspace)
		if err != nil {
			return err
		}
		ps, err := port.LiveProjects(r, it.Place.Workspace)
		if err != nil {
			return err
		}
		if it.Kind == item.Epic {
			out = projectCandidates(it, ps, w.Prefix)
			return nil
		}
		out, err = itemCandidates(r, it, ps, w.Prefix)
		return err
	})
	return out, err
}

// projectCandidates は、Epic の移し先になる、いまのプロジェクト以外のプロジェクト。
func projectCandidates(it item.Item, ps []project.Project, prefix string) []MoveCandidate {
	var out []MoveCandidate
	for _, p := range ps {
		if p.ID != it.Place.Project {
			out = append(out, MoveCandidate{Project: &p, ProjectID: p.ID, Display: p.DisplayID(prefix)})
		}
	}
	return out
}

// itemCandidates は、Epic 以外の移し先になるアイテムを、プロジェクトの並び → ツリーの並びで返す。置ける種類(CanPlace)だけが
// 候補になるので、自分の子孫は入らない(Step は親になれず、Epic の子孫は Epic の下に置けない)。
func itemCandidates(r port.Reader, it item.Item, ps []project.Project, prefix string) ([]MoveCandidate, error) {
	var out []MoveCandidate
	for _, p := range ps {
		all, err := port.LiveItems(r, p.ID)
		if err != nil {
			return nil, err
		}
		for _, c := range treeOrder(all) {
			if c.ID == it.ID || (it.Place.Parent != nil && c.ID == *it.Place.Parent) || !item.CanPlace(it.Kind, &c.Kind) {
				continue
			}
			out = append(out, MoveCandidate{Item: &c, ProjectID: p.ID, ProjectTitle: p.Title, Display: c.DisplayID(prefix)})
		}
	}
	return out, nil
}

// treeOrder は、アイテムをツリーの並び(Epic の順、それぞれの下を深さ優先で兄弟の順)にする。
func treeOrder(all []item.Item) []item.Item {
	roots := siblings(all, nil)
	item.SortSiblings(roots)
	var out []item.Item
	for _, root := range roots {
		out = append(out, root)
		out = append(out, item.Descendants(root.ID, all)...)
	}
	return out
}
