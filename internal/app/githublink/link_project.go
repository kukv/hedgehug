package githublink

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// ProjectLinkRow は、プロジェクトの概要の「PR と Issue」の 1 行(02「概要」)。
type ProjectLinkRow struct {
	LinkRow
	Item    item.ID
	Display string
}

// ProjectLinks は、プロジェクトの下の削除済みでないアイテムのリンクを、アイテムの番号順、同じアイテムの中はリンクした順に返す。
// GitHub には問い合わせない。プロジェクトがないか削除済みなら port.ErrNotFound。
func (l Links) ProjectLinks(ctx context.Context, store port.StoreID, id project.ID) ([]ProjectLinkRow, error) {
	var out []ProjectLinkRow
	err := l.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		out, err = projectLinks(r, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	checks, err := l.Checks.Read(ctx, store)
	for i := range out {
		out[i].Checks = checks[out[i].Key()]
	}
	return out, err
}

// projectLinks は、プロジェクトの下の、削除済みでないアイテムのリンクを読む。プロジェクトが削除済みなら
// port.ErrNotFound(A4b-4)。
func projectLinks(r port.Reader, id project.ID) ([]ProjectLinkRow, error) {
	p, err := port.LiveProject(r, id)
	if err != nil {
		return nil, err
	}
	ws, err := r.Workspaces().Get(p.Place.Workspace)
	if err != nil {
		return nil, err
	}
	its, err := port.LiveItems(r, id)
	if err != nil {
		return nil, err
	}
	var out []ProjectLinkRow
	for _, it := range its {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: it.ID})
		if err != nil {
			return nil, err
		}
		for _, link := range itemlink.FromLog(log).Links {
			out = append(out, ProjectLinkRow{LinkRow: LinkRow{Link: link}, Item: it.ID, Display: it.DisplayID(ws.Prefix)})
		}
	}
	return out, nil
}
