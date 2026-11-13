package projects

import (
	"context"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// RemoveRepo は、リポジトリを 1 つ外す(02「リポジトリの追加と削除」。確認はしない)。ないものは何も保存しない。
// 行は、保存してある綴りで書く(大文字と小文字を区別しない)。アクティビティは「{タイトル} からリポジトリ {owner/name} を削除」。
func (s Service) RemoveRepo(ctx context.Context, store port.StoreID, id project.ID, input string) (project.Project, error) {
	repo := strings.TrimSpace(input)
	p, err := s.editable(ctx, store, id)
	if err != nil {
		return project.Project{}, err
	}
	summary := fmt.Sprintf("%s からリポジトリ %s を削除", label(p.Title), repo)
	return s.save(ctx, store, id, summary, func(_ port.Tx, cur project.Project) ([]change.Draft, error) {
		i := project.IndexRepo(cur.Repos, repo)
		if i < 0 {
			return nil, nil
		}
		return []change.Draft{change.NewRemove("repos", cur.Repos[i])}, nil
	})
}
