package projects

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// DeletePreview は、プロジェクトの削除の確認ダイアログの中身(02「名前の変更・移動・削除」の削除。A4b-7)。
// Items は、プロジェクトの下の削除されていないアイテム(Epic から Step まで全部)の数。
type DeletePreview struct {
	Items         int
	RetentionDays int
}

// liveItems は、id のプロジェクトと、その下の削除されていないアイテムすべてを返す。
// プロジェクトがない・削除済みなら port.ErrNotFound。
func liveItems(r port.Reader, id project.ID) (project.Project, []item.Item, error) {
	p, err := port.LiveProject(r, id)
	if err != nil {
		return project.Project{}, nil, err
	}
	its, err := port.LiveItems(r, id)
	return p, its, err
}

// DeletePreview は、id を削除したときに一緒に消えるアイテムの数と、保持日数を返す。
// プロジェクトがない・削除済みなら port.ErrNotFound。
func (s Service) DeletePreview(ctx context.Context, store port.StoreID, id project.ID) (DeletePreview, error) {
	var out DeletePreview
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		_, its, err := liveItems(r, id)
		if err != nil {
			return err
		}
		out.Items = len(its)
		out.RetentionDays, err = r.RetentionDays()
		return err
	})
	return out, err
}

// Delete は、プロジェクトと、その下の削除されていないアイテムすべてを、同じ 1 回の保存で削除する
// (02「名前の変更・移動・削除」の削除)。書くのは、それぞれのログの `delete`・`deleted` の行 1 つで、
// プロジェクトは root: true、アイテムは root: false(A4b-1)。すでに削除済みのアイテムには書かない。
// プロジェクト別のルールと専用ルールは消さない。アクティビティは「プロジェクト「{タイトル}」を削除」。
// プロジェクトがない・削除済みなら port.ErrNotFound。
func (s Service) Delete(ctx context.Context, store port.StoreID, id project.ID) error {
	var title string
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		p, _, err := liveItems(r, id)
		title = p.Title
		return err
	})
	if err != nil {
		return err
	}
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: change.User(me), Summary: fmt.Sprintf("プロジェクト「%s」を削除", label(title))}
	return s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		_, its, err := liveItems(tx, id)
		if err != nil {
			return err
		}
		if err := tx.WriteProjects().Append(id, change.NewDeleteEntity(true)); err != nil {
			return err
		}
		for _, it := range its {
			if err := tx.WriteItems().Append(it.ID, change.NewDeleteEntity(false)); err != nil {
				return err
			}
		}
		return nil
	})
}
