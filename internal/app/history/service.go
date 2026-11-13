// Package history は、アイテムとプロジェクトの履歴(02「履歴」)を読む操作を持つ。
package history

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	dhistory "github.com/kukv/hedgehug/internal/domain/history"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Service は、履歴を読む口。StoreList は、同じ Store の中の移動の行に出す Store 名を引くため(A5a-17)。
type Service struct {
	Stores    port.Stores
	StoreList port.StoreList
	Clock     port.Clock
}

// Item は、アイテムの履歴を新しい順で返す。なければ port.ErrNotFound。値の名前は、行を書いたときにアイテムがいた
// ワークスペースの設定で引く(いまのワークスペースと、同じ Store のほかのワークスペース)。
// ワークスペースのログに同じ操作の delete の行があるアイテムの行は出さない(B2-9)。
func (s Service) Item(ctx context.Context, store port.StoreID, id item.ID) ([]dhistory.Row, error) {
	name, err := s.storeName(ctx, store)
	if err != nil {
		return nil, err
	}
	var rows []dhistory.Row
	err = s.Stores.Read(ctx, store, func(r port.Reader) error {
		it, err := port.LiveItem(r, id)
		if err != nil {
			return err
		}
		st, err := r.Workspaces().Settings(it.Place.Workspace)
		if err != nil {
			return err
		}
		wsLog, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: it.Place.Workspace})
		if err != nil {
			return err
		}
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: id})
		if err != nil {
			return err
		}
		ms, err := r.Members().List()
		if err != nil {
			return err
		}
		origin, err := readOrigin(r, id)
		if err != nil {
			return err
		}
		origin.StoreName = name
		rows = dhistory.ItemRows(log, dhistory.Context{
			Settings: st, Names: dhistory.NamesOf(ms), Hidden: dhistory.HiddenOps(wsLog), Today: s.Clock.Now(), Origin: origin,
		})
		return nil
	})
	return rows, err
}

// storeName は、表示中の Store id の名前。なければ空。
func (s Service) storeName(ctx context.Context, id port.StoreID) (string, error) {
	infos, err := s.StoreList.List(ctx)
	if err != nil {
		return "", err
	}
	for _, info := range infos {
		if info.ID == id {
			return info.Name, nil
		}
	}
	return "", nil
}

// readOrigin は、同じ Store の中の移動の材料: すべてのワークスペースの設定と名前、すべてのプロジェクト(削除済みを含む)の
// 名前、アイテム id を Target に持つ転送記録。
func readOrigin(r port.Reader, id item.ID) (dhistory.Origin, error) {
	wss, err := r.Workspaces().List()
	if err != nil {
		return dhistory.Origin{}, err
	}
	settings := map[workspace.ID]workspace.Settings{}
	names := map[workspace.ID]string{}
	titles := map[project.ID]string{}
	for _, w := range wss {
		if settings[w.ID], err = r.Workspaces().Settings(w.ID); err != nil {
			return dhistory.Origin{}, err
		}
		names[w.ID] = w.Name
		ps, err := r.Projects().List(w.ID)
		if err != nil {
			return dhistory.Origin{}, err
		}
		for _, p := range ps {
			titles[p.ID] = p.Title
		}
	}
	recs, err := r.Transfers().List()
	if err != nil {
		return dhistory.Origin{}, err
	}
	var mine []transfer.Record
	for _, rec := range recs {
		if rec.Entity == transfer.EntityItem && rec.Target == id {
			mine = append(mine, rec)
		}
	}
	return dhistory.Origin{
		Settings:      func(id workspace.ID) (workspace.Settings, bool) { st, ok := settings[id]; return st, ok },
		WorkspaceName: func(id workspace.ID) (string, bool) { n, ok := names[id]; return n, ok },
		ProjectTitle:  func(id project.ID) (string, bool) { t, ok := titles[id]; return t, ok },
		Transfers:     mine,
	}, nil
}

// Project は、プロジェクトの履歴を新しい順で返す。なければ port.ErrNotFound。
func (s Service) Project(ctx context.Context, store port.StoreID, id project.ID) ([]dhistory.Row, error) {
	var rows []dhistory.Row
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		if _, err := port.LiveProject(r, id); err != nil {
			return err
		}
		log, err := r.Changes().Of(change.Ref{Kind: change.KindProject, ID: id})
		if err != nil {
			return err
		}
		ms, err := r.Members().List()
		if err != nil {
			return err
		}
		rows = dhistory.ProjectRows(log, dhistory.ProjectContext{Names: dhistory.NamesOf(ms), Today: s.Clock.Now()})
		return nil
	})
	return rows, err
}
