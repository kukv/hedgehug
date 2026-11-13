// Package history は、アイテムとプロジェクトの履歴(02「履歴」)を読む操作を持つ。
package history

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	dhistory "github.com/kukv/hedgehug/internal/domain/history"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// Service は、履歴を読む口。
type Service struct {
	Stores port.Stores
	Clock  port.Clock
}

// Item は、アイテムの履歴を新しい順で返す。なければ port.ErrNotFound。値の名前は、いまのワークスペースの設定で引く。
// ワークスペースのログに同じ操作の delete の行があるアイテムの行は出さない(B2-9)。
func (s Service) Item(ctx context.Context, store port.StoreID, id item.ID) ([]dhistory.Row, error) {
	var rows []dhistory.Row
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		it, err := r.Items().Get(id)
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
		rows = dhistory.ItemRows(log, dhistory.Context{
			Settings: st, Names: dhistory.NamesOf(ms), Hidden: dhistory.HiddenOps(wsLog), Today: s.Clock.Now(),
		})
		return nil
	})
	return rows, err
}

// Project は、プロジェクトの履歴を新しい順で返す。なければ port.ErrNotFound。
func (s Service) Project(ctx context.Context, store port.StoreID, id project.ID) ([]dhistory.Row, error) {
	var rows []dhistory.Row
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		if _, err := r.Projects().Get(id); err != nil {
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
