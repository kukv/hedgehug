package repairing

import (
	"context"
	"errors"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/repair"
)

// matchProjects は、親とプロジェクトが違う子を、親のプロジェクトへ 1 件ずつ移す(決定 C3a-16)。
// 番号と親は変えない。1 件ごとに 1 回保存し、なくなるまで読み直して繰り返す。
func matchProjects(ctx context.Context, w port.RepairWriter) error {
	var prev repair.Mismatch
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		all, err := readAllItems(w)
		if err != nil {
			return err
		}
		m, ok := repair.ProjectMismatch(all)
		if !ok {
			return nil
		}
		if m == prev {
			return errors.New("親とプロジェクトが違う子の修復が進みません")
		}
		prev = m
		if err := moveToProject(w, m); err != nil {
			return err
		}
	}
}

// moveToProject は、m.Child のプロジェクトだけを m.Project にする修復を 1 回保存する。
func moveToProject(w port.RepairWriter, m repair.Mismatch) error {
	var it item.Item
	var summary string
	err := w.Read(func(r port.Reader) error {
		var err error
		if it, err = r.Items().Get(m.Child); err != nil {
			return err
		}
		ws, err := r.Workspaces().Get(it.Place.Workspace)
		if err != nil {
			return err
		}
		p, err := r.Projects().Get(m.Project)
		if err != nil {
			return err
		}
		summary = it.DisplayID(ws.Prefix) + " をプロジェクト「" + item.NormalizeTitle(p.Title) + "」に移動(親に合わせる)"
		return nil
	})
	if err != nil {
		return err
	}
	to := it.Place
	to.Project = m.Project
	op := port.Operation{Actor: syncActor, Summary: summary}
	return w.Write(op, func(tx port.Tx) error {
		return tx.WriteItems().Append(it.ID, change.NewSet(placeField, it.Place, to))
	})
}
