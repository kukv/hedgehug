package workspaces

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// List は、store のワークスペースを返す。
func (s Service) List(ctx context.Context, store port.StoreID) ([]workspace.Workspace, error) {
	var out []workspace.Workspace
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		out, err = r.Workspaces().List()
		return err
	})
	return out, err
}
