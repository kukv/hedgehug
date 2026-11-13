package projects

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// AddRepo は、リポジトリを 1 つ結びつける(02「リポジトリの追加と削除」)。owner/name の形でなければ断り、
// すでにあるもの(大文字と小文字を区別しない)は、保存の中で読み直した一覧で見て断る。
// 履歴には残らず、アクティビティは「{タイトル} にリポジトリ {owner/name} を追加」。
func (s Service) AddRepo(ctx context.Context, store port.StoreID, id project.ID, input string) (project.Project, error) {
	repo, err := project.NormalizeRepo(input)
	if err != nil {
		return project.Project{}, err
	}
	p, err := s.editable(ctx, store, id)
	if err != nil {
		return project.Project{}, err
	}
	summary := fmt.Sprintf("%s にリポジトリ %s を追加", label(p.Title), repo)
	return s.save(ctx, store, id, summary, func(_ port.Tx, cur project.Project) ([]change.Draft, error) {
		if project.IndexRepo(cur.Repos, repo) >= 0 {
			return nil, refusal.New("もう追加されています")
		}
		return []change.Draft{change.NewAdd("repos", repo)}, nil
	})
}
