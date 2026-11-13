package worklogs

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/worklog"
)

// List は、アイテムの作業時間の記録を古い順にすべて返す(02「作業時間」)。アイテムがなければ port.ErrNotFound。
func (s Service) List(ctx context.Context, store port.StoreID, id item.ID) ([]View, error) {
	var out []View
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		if _, err := display(r, id); err != nil {
			return err
		}
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: id})
		if err != nil {
			return err
		}
		nameOf, err := names(r)
		if err != nil {
			return err
		}
		for _, e := range worklog.FromLog(log) {
			out = append(out, View{Entry: e, ByName: nameOf(e.By)})
		}
		return nil
	})
	return out, err
}
