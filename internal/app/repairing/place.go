package repairing

import (
	"context"
	"errors"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
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

// moveToProject は、m.Child のプロジェクトを m.Project にする修復を 1 回保存する。
// 移動先が使わないステータスなら、同じ分類の既定のステータスに置き換える(06、決定 C3a-16)。
func moveToProject(w port.RepairWriter, m repair.Mismatch) error {
	var it item.Item
	var summary string
	var replace []change.Draft
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
		if replace, err = replaceStatus(r, it, m.Project); err != nil {
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
	ds := append([]change.Draft{change.NewSet(placeField, it.Place, to)}, replace...)
	return w.Write(op, func(tx port.Tx) error {
		return tx.WriteItems().Append(it.ID, ds...)
	})
}

// replaceStatus は、プロジェクト p が it のステータスを使わないとき、ステータスを同じ分類の既定へ置き換える行を返す。
func replaceStatus(r port.Reader, it item.Item, p project.ID) ([]change.Draft, error) {
	if it.Status == "" {
		return nil, nil
	}
	st, err := r.Workspaces().Settings(it.Place.Workspace)
	if err != nil {
		return nil, err
	}
	log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: it.Place.Workspace})
	if err != nil && !errors.Is(err, port.ErrNotFound) {
		return nil, err
	}
	if projectrule.Uses(st, projectrule.FromLog(log).Of(p), it.Status) {
		return nil, nil
	}
	for _, v := range st.Statuses {
		if v.ID != it.Status {
			continue
		}
		if to, ok := st.CategoryDefaults[v.Category]; ok {
			return []change.Draft{change.NewSet("status", it.Status, to)}, nil
		}
	}
	return nil, nil
}
