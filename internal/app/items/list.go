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
		if _, err := r.Projects().Get(p); err != nil {
			return err
		}
		var err error
		out, err = r.Items().List(p)
		return err
	})
	return out, err
}
