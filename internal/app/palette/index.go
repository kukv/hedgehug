package palette

import (
	"context"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

// Index は、表示中のすべての Store(閲覧のみを含む)の、削除されていないアイテム・プロジェクトを、旧 ID と一緒に返す
// (A5c-3)。旧 ID の転送記録は、すべての Store の分を集めて探す(別の Store へ移しても内部 ID は変わらない。A5c-4)。
func (s Service) Index(ctx context.Context) (Index, error) {
	snaps, err := s.snapshots(ctx)
	if err != nil {
		return Index{}, err
	}
	var records []transfer.Record
	for _, snap := range snaps {
		records = append(records, snap.records...)
	}
	var idx Index
	for _, snap := range snaps {
		for _, w := range snap.workspaces {
			for _, p := range w.projects {
				idx.Projects = append(idx.Projects, indexProject(snap.info.ID, w, p.project, records))
				for _, it := range p.items {
					idx.Items = append(idx.Items, indexItem(snap.info.ID, w, p.project, it, records))
				}
			}
		}
	}
	return idx, nil
}

func indexProject(store port.StoreID, w wsSnap, p project.Project, records []transfer.Record) IndexProject {
	display := p.DisplayID(w.ws.Prefix)
	return IndexProject{
		Store: store, Workspace: w.ws.ID, Project: p.ID, Display: display, Title: p.Title, WorkspaceName: w.ws.Name,
		Aliases: aliases(transfer.Aliases(records, transfer.EntityProject, p.ID, w.old, p.Place.Number, "-P"), display),
	}
}

func indexItem(store port.StoreID, w wsSnap, p project.Project, it item.Item, records []transfer.Record) IndexItem {
	display := it.DisplayID(w.ws.Prefix)
	return IndexItem{
		Store: store, Workspace: w.ws.ID, Project: p.ID, Item: it.ID, Number: it.Place.Number,
		Display: display, Title: it.Title, ProjectTitle: p.Title,
		Aliases: aliases(transfer.Aliases(records, transfer.EntityItem, it.ID, w.old, it.Place.Number, "-"), display),
	}
}

// aliases は、旧 ID の一覧から、いまの ID current と同じものを除く(移して戻したときの記録の Old)。
func aliases(all []transfer.Alias, current string) []transfer.Alias {
	return slices.DeleteFunc(all, func(a transfer.Alias) bool { return a.ID == current })
}
