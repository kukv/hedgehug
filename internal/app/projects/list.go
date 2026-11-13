package projects

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// List は、ワークスペースのプロジェクトを並び順で返す。ワークスペースがなければ port.ErrNotFound。
func (s Service) List(ctx context.Context, store port.StoreID, ws workspace.ID) ([]project.Project, error) {
	var out []project.Project
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		if _, err := port.LiveWorkspace(r, ws); err != nil {
			return err
		}
		var err error
		out, err = port.LiveProjects(r, ws)
		return err
	})
	return out, err
}
