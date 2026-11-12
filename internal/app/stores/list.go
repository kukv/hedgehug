// Package stores は、このマシンの Store の一覧の操作を持つ。Store の追加・削除はトラック G が足す。
package stores

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
)

// Service は、Store の一覧の操作が使う口。
type Service struct {
	Stores port.StoreList
}

// All は、表示中の Store を、サイドバーの並びで返す。
func (s Service) All(ctx context.Context) ([]port.StoreInfo, error) {
	return s.Stores.List(ctx)
}
