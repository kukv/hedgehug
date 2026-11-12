package apptest

import (
	"context"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
)

// StoreList は、Stores に並べた Store を、その順でそのまま返す偽物。
type StoreList struct {
	Stores []port.StoreInfo
}

func (l StoreList) List(context.Context) ([]port.StoreInfo, error) {
	return slices.Clone(l.Stores), nil
}
