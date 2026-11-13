package projects

import (
	"context"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// Rename は、タイトルを変える(02「概要の編集」)。前後の空白を除いた形で保存し、前と同じなら何も保存しない。
// アクティビティは「{変える前のタイトル} のタイトルを {新しいタイトル} に変更」。
func (s Service) Rename(ctx context.Context, store port.StoreID, id project.ID, title string) (project.Project, error) {
	title = strings.TrimSpace(title)
	p, err := s.Get(ctx, store, id)
	if err != nil {
		return project.Project{}, err
	}
	summary := fmt.Sprintf("%s のタイトルを %s に変更", label(p.Title), label(title))
	return s.save(ctx, store, id, summary, func(tx port.Tx, cur project.Project) ([]change.Draft, error) {
		siblings, err := tx.Projects().List(cur.Place.Workspace)
		if err != nil {
			return nil, err
		}
		if err := project.CheckRename(title, cur.ID, siblings); err != nil {
			return nil, err
		}
		if cur.Title == title {
			return nil, nil
		}
		return []change.Draft{change.NewSet("title", cur.Title, title)}, nil
	})
}
