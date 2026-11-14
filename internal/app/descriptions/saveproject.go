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
		p, err := port.EditableProject(r, id)
		title = p.Title
		return err
	})
	if err != nil {
		return Result{}, err
	}
	// アクティビティの 1 行は 1 行でなければならないので、改行を含む空白を 1 つの空白にまとめる。
	label := strings.Join(strings.Fields(title), " ")
	return s.save(ctx, store, label+" の説明を変更", base, text, slot{
		ref:    change.Ref{Kind: change.KindProject, ID: id},
		target: func(r port.Reader) (port.Notice, error) { return projectTarget(r, id) },
		revive: func(ctx context.Context, tx port.Tx) (*change.Deletion, error) {
			p, err := port.EditableProject(tx, id)
			if err != nil {
				return nil, err
			}
			return p.Deleted, port.ReviveProject(ctx, tx, id)
		},
		read: func(r port.Reader) (string, error) { return r.Projects().Description(id) },
		write: func(tx port.Tx, merged string) error {
			return errors.Join(tx.WriteProjects().Describe(id, merged), tx.WriteProjects().Append(id, change.NewDesc()))
		},
	})
}

// projectTarget は、プロジェクト id を指す通知の下地(ワークスペース・プロジェクト・画面の ID)。
func projectTarget(r port.Reader, id project.ID) (port.Notice, error) {
	p, err := port.EditableProject(r, id)
	if err != nil {
		return port.Notice{}, err
	}
	w, err := r.Workspaces().Get(p.Place.Workspace)
	if err != nil {
		return port.Notice{}, err
	}
	return port.Notice{Workspace: w.ID, Project: id, Display: p.DisplayID(w.Prefix)}, nil
}
