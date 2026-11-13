package items

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// List は、プロジェクトのすべてのアイテムを番号の順で返す。プロジェクトがなければ port.ErrNotFound。
func (s Service) List(ctx context.Context, store port.StoreID, p project.ID) ([]item.Item, error) {
	var out []item.Item
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		if _, err := port.LiveProject(r, p); err != nil {
			return err
		}
		var err error
		if out, err = port.LiveItems(r, p); err != nil {
			return err
		}
		blocked, err := blockedSet(r)
		for i := range out {
			out[i].Blocked = blocked[out[i].ID]
		}
		return err
	})
	return out, err
}
