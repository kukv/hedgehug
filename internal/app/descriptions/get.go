package descriptions

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// GetItem は、アイテムの説明の本文を返す。説明がなければ ""。アイテムがないか削除済みなら port.ErrNotFound
// (説明の口は削除済みでも本文を返すので、先に確かめる。A4b-4)。
func (s Service) GetItem(ctx context.Context, store port.StoreID, id item.ID) (string, error) {
	var text string
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		if _, err := port.LiveItem(r, id); err != nil {
			return err
		}
		var err error
		text, err = r.Items().Description(id)
		return err
	})
	return text, err
}

// GetProject は、プロジェクトの説明の本文を返す。説明がなければ ""。プロジェクトがないか削除済みなら
// port.ErrNotFound(GetItem と同じく先に確かめる。A4b-4)。
func (s Service) GetProject(ctx context.Context, store port.StoreID, id project.ID) (string, error) {
	var text string
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		if _, err := port.LiveProject(r, id); err != nil {
			return err
		}
		var err error
		text, err = r.Projects().Description(id)
		return err
	})
	return text, err
}
