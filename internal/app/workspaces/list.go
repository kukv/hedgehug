package workspaces

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// List は、store のワークスペースを並びの順(rank、同じなら内部 ID の順。rank のないものは先頭)に返す(B2-7)。
func (s Service) List(ctx context.Context, store port.StoreID) ([]workspace.Workspace, error) {
	var out []workspace.Workspace
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		list, err := orderedWorkspaces(r)
		for _, w := range list {
			out = append(out, w.Workspace)
		}
		return err
	})
	return out, err
}
