package projects

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// Get は、プロジェクトを 1 つ返す。なければ port.ErrNotFound。
func (s Service) Get(ctx context.Context, store port.StoreID, id project.ID) (project.Project, error) {
	var p project.Project
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		p, err = port.LiveProject(r, id)
		return err
	})
	return p, err
}
