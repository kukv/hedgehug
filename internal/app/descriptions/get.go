package descriptions

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// GetItem は、アイテムの説明の本文を返す。説明がなければ ""。アイテムがなければ port.ErrNotFound。
func (s Service) GetItem(ctx context.Context, store port.StoreID, id item.ID) (string, error) {
	var text string
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		text, err = r.Items().Description(id)
		return err
	})
	return text, err
}

// GetProject は、プロジェクトの説明の本文を返す。決まりは GetItem と同じ。
func (s Service) GetProject(ctx context.Context, store port.StoreID, id project.ID) (string, error) {
	var text string
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		text, err = r.Projects().Description(id)
		return err
	})
	return text, err
}
