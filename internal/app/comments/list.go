package comments

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/comment"
)

// List は、持ち主のコメントを古い順にすべて返す(02「コメント」)。持ち主がなければ port.ErrNotFound。
func (s Service) List(ctx context.Context, store port.StoreID, t Target) ([]View, error) {
	var out []View
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		if _, err := display(r, t); err != nil {
			return err
		}
		log, err := r.Changes().Of(t.ref())
		if err != nil {
			return err
		}
		nameOf, err := names(r)
		if err != nil {
			return err
		}
		for _, c := range comment.FromLog(log) {
			out = append(out, View{Comment: c, ByName: nameOf(c.By)})
		}
		return nil
	})
	return out, err
}
