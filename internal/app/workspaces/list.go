package workspaces

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Listed は、一覧のワークスペースと、その旧い接頭辞(辞書順。画面のアドレスの旧い接頭辞のため。A5a-21)。
type Listed struct {
	workspace.Workspace
	OldPrefixes []string
}

// List は、store のワークスペースを並びの順(rank、同じなら内部 ID の順。rank のないものは先頭)に返す(B2-7)。
func (s Service) List(ctx context.Context, store port.StoreID) ([]Listed, error) {
	var out []Listed
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		list, err := orderedWorkspaces(r)
		for _, w := range list {
			out = append(out, Listed{w.Workspace, w.oldPrefixes})
		}
		return err
	})
	return out, err
}
