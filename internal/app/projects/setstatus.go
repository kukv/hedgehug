package projects

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// SetStatus は、ステータスを変える(02「概要の編集」)。下のアイテムには影響しない。前と同じなら何も保存しない。
func (s Service) SetStatus(ctx context.Context, store port.StoreID, id project.ID, status project.Status) (project.Project, error) {
	if !project.ValidStatus(status) {
		return project.Project{}, fmt.Errorf("unknown project status %q", status)
	}
	p, err := s.Get(ctx, store, id)
	if err != nil {
		return project.Project{}, err
	}
	summary := fmt.Sprintf("%s のステータスを %s に変更", label(p.Title), status)
	return s.save(ctx, store, id, summary, func(_ port.Tx, cur project.Project) ([]change.Draft, error) {
		if cur.Status == status {
			return nil, nil
		}
		return []change.Draft{change.NewSet("status", cur.Status, status)}, nil
	})
}
