package descriptions

import (
	"context"
	"errors"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// SaveProject は、プロジェクトの説明を保存する。編集を始めたときの説明(base)と行ごとに合わせるのは SaveItem と同じで、
// アクティビティは「{タイトル} の説明を変更」(A4a-18)。
func (s Service) SaveProject(ctx context.Context, store port.StoreID, id project.ID, base, text string) (Result, error) {
	var title string
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		p, err := r.Projects().Get(id)
		title = p.Title
		return err
	})
	if err != nil {
		return Result{}, err
	}
	// アクティビティの 1 行は 1 行でなければならないので、改行を含む空白を 1 つの空白にまとめる。
	label := strings.Join(strings.Fields(title), " ")
	return s.save(ctx, store, label+" の説明を変更", base, text, slot{
		read: func(r port.Reader) (string, error) { return r.Projects().Description(id) },
		write: func(tx port.Tx, merged string) error {
			return errors.Join(tx.WriteProjects().Describe(id, merged), tx.WriteProjects().Append(id, change.NewDesc()))
		},
	})
}
