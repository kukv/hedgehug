// Package workspaces は、ワークスペースの設定の操作(仕様 03)を持つ。1 ファイル 1 操作。
package workspaces

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// Service は、ワークスペースの操作が使う口。
type Service struct {
	Stores   port.Stores
	IDs      port.IDs
	Identity port.Identity
}

// me は、store の「あなた」を実行者にする。ワークスペースの操作が共通で使う。
func (s Service) me(ctx context.Context, store port.StoreID) (change.Actor, error) {
	login, err := s.Identity.Me(ctx, store)
	if err != nil {
		return change.Actor{}, err
	}
	return change.User(login), nil
}
