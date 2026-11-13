// Package stores は、このマシンの Store の一覧を読む操作を持つ。
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
