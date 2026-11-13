package items

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// NextNumber は、ws で次に作るアイテムの番号を返す(作成ダイアログの ID の予告。02)。目安で、実際の番号は
// 保存のときに決まる。ワークスペースがなければ port.ErrNotFound。
func (s Service) NextNumber(ctx context.Context, store port.StoreID, ws workspace.ID) (int, error) {
	n := 0
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		if _, err := port.LiveWorkspace(r, ws); err != nil {
			return err
		}
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws})
		if err != nil {
			return err
		}
		n = numbering.Next(log, numbering.Items)
		return nil
	})
	return n, err
}

// FindByNumber は、ws の番号 number のアイテムを返す(画面のアドレスの HH-12 の解決。01)。
// なければ port.ErrNotFound。旧い接頭辞と転送の記録はたどらない(B1・A5)。
func (s Service) FindByNumber(ctx context.Context, store port.StoreID, ws workspace.ID, number int) (item.Item, error) {
	var found item.Item
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		ps, err := port.LiveProjects(r, ws)
		if err != nil {
			return err
		}
		for _, p := range ps {
			its, err := port.LiveItems(r, p.ID)
			if err != nil {
				return err
			}
			for _, it := range its {
				if it.Place.Workspace == ws && it.Place.Number == number {
					found = it
					blocked, err := blockedSet(r)
					found.Blocked = blocked[it.ID]
					return err
				}
			}
		}
		return port.ErrNotFound
	})
	return found, err
}
