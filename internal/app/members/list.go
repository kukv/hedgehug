// Package members は、Store のメンバーの操作を持つ。メンバーへの追加と表示名の変更はトラック G が足す。
package members

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/member"
)

// Service は、メンバーの操作が使う口。
type Service struct {
	Stores   port.Stores
	Identity port.Identity
}

// List は、store のメンバーを返す。
func (s Service) List(ctx context.Context, store port.StoreID) ([]member.Member, error) {
	var out []member.Member
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		out, err = r.Members().List()
		return err
	})
	return out, err
}

// Me は、store での「あなた」のログイン名を返す(01「「あなた」の決まり方」)。
func (s Service) Me(ctx context.Context, store port.StoreID) (string, error) {
	return s.Identity.Me(ctx, store)
}
