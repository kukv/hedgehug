package workspaces

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// DeletePreview は、ワークスペースの削除の確認ダイアログの中身(03「削除」。B5b-9)。
// Projects・Items は、ワークスペースの削除されていないプロジェクトと、その下の削除されていないアイテムの数。
type DeletePreview struct {
	Projects      int
	Items         int
	RetentionDays int
}

// liveContents は、ws の名前と、削除されていないプロジェクトと、その下の削除されていないアイテム(プロジェクトと同じ並び)を返す。
// ワークスペースがない・削除済みなら port.ErrNotFound。
func liveContents(r port.Reader, ws workspace.ID) (string, []project.Project, [][]item.Item, error) {
	w, err := port.LiveWorkspace(r, ws)
	if err != nil {
		return "", nil, nil, err
	}
	ps, err := port.LiveProjects(r, ws)
	if err != nil {
		return "", nil, nil, err
	}
	its := make([][]item.Item, len(ps))
	for i, p := range ps {
		if its[i], err = port.LiveItems(r, p.ID); err != nil {
			return "", nil, nil, err
		}
	}
	return w.Name, ps, its, nil
}

// DeletePreview は、ws を削除したときに一緒に消えるプロジェクトとアイテムの数と、保持日数を返す。
// ワークスペースがない・削除済みなら port.ErrNotFound。
func (s Service) DeletePreview(ctx context.Context, store port.StoreID, ws workspace.ID) (DeletePreview, error) {
	var out DeletePreview
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		_, ps, its, err := liveContents(r, ws)
		if err != nil {
			return err
		}
		out.Projects = len(ps)
		for _, list := range its {
			out.Items += len(list)
		}
		out.RetentionDays, err = r.RetentionDays()
		return err
	})
	return out, err
}

// Delete は、ワークスペースと、その削除されていないプロジェクトとアイテムを、同じ 1 回の保存で削除する
// (03「削除」。B5b-1)。書くのは、それぞれのログの `delete`・`deleted` の行 1 つで、ワークスペースは root: true、
// プロジェクト・アイテムは root: false。すでに削除済みのプロジェクト・アイテムには書かない。
// ワークスペースのログの設定には行を足さない(B5b-2)。アクティビティは「ワークスペース「{名前}」を削除」。
// ワークスペースがない・削除済みなら port.ErrNotFound。
func (s Service) Delete(ctx context.Context, store port.StoreID, ws workspace.ID) error {
	var name string
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		name, _, _, err = liveContents(r, ws)
		return err
	})
	if err != nil {
		return err
	}
	actor, err := s.me(ctx, store)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: actor, Summary: fmt.Sprintf("ワークスペース「%s」を削除", oneLine(name))}
	return s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		_, ps, its, err := liveContents(tx, ws)
		if err != nil {
			return err
		}
		return appendDeletions(tx, ws, ps, its)
	})
}

// appendDeletions は、ワークスペース(root: true)と、プロジェクト・アイテム(root: false)に削除の行を書く。
func appendDeletions(tx port.Tx, ws workspace.ID, ps []project.Project, its [][]item.Item) error {
	if err := tx.WriteWorkspaces().Append(ws, change.NewDeleteEntity(true)); err != nil {
		return err
	}
	for i, p := range ps {
		if err := tx.WriteProjects().Append(p.ID, change.NewDeleteEntity(false)); err != nil {
			return err
		}
		for _, it := range its[i] {
			if err := tx.WriteItems().Append(it.ID, change.NewDeleteEntity(false)); err != nil {
				return err
			}
		}
	}
	return nil
}
