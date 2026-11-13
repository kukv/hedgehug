package items

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// Get は、アイテムを 1 つ返す。なければ port.ErrNotFound。
func (s Service) Get(ctx context.Context, store port.StoreID, id item.ID) (item.Item, error) {
	var it item.Item
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		if it, err = r.Items().Get(id); err != nil {
			return err
		}
		blocked, err := blockedSet(r)
		it.Blocked = blocked[it.ID]
		return err
	})
	return it, err
}
