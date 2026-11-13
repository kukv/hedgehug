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

// editable は、Get と同じで、削除済みのうち利用者が選んで削除したもの(root: true)も返す。
// 別の画面で削除されたプロジェクトへの保存が、削除を取り消して保存するために読む(06「他の書き手との重なり」)。
func (s Service) editable(ctx context.Context, store port.StoreID, id project.ID) (project.Project, error) {
	var p project.Project
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		p, err = port.EditableProject(r, id)
		return err
	})
	return p, err
}
