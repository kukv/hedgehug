package repairing

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/repair"
)

// cascadeDeleted は、削除済みの親の下に残ったアイテムを、親の削除の op で削除済みにする(決定 C3a-10・C3a-11)。
// 1 件ごとに 1 回保存する。子の子は、読み直した次の回で見つかるので、なくなるまで繰り返す。
func cascadeDeleted(ctx context.Context, w port.RepairWriter) error {
	for {
		cs, display, err := readCascades(w)
		if err != nil || len(cs) == 0 {
			return err
		}
		for _, c := range cs {
			if err := ctx.Err(); err != nil {
				return err
			}
			op := port.Operation{Actor: syncActor, Summary: display[c.Child] + " を削除"}
			err := w.Write(op, func(tx port.Tx) error {
				return tx.WriteItems().Append(c.Child, change.NewDeleteEntity(false).InOp(c.Op))
			})
			if err != nil {
				return err
			}
		}
	}
}

// readCascades は、いまの Store から、削除に入れるアイテムと、アイテムの画面の ID を読む。
func readCascades(w port.RepairWriter) ([]repair.Cascade, map[item.ID]string, error) {
	var items []item.Item
	var projects []project.Project
	display := map[item.ID]string{}
	err := w.Read(func(r port.Reader) error {
		wss, err := r.Workspaces().List()
		if err != nil {
			return err
		}
		for _, ws := range wss {
			ps, err := r.Projects().List(ws.ID)
			if err != nil {
				return err
			}
			projects = append(projects, ps...)
			its, err := itemsOf(r, ps, ws.Prefix, display)
			if err != nil {
				return err
			}
			items = append(items, its...)
		}
		return nil
	})
	return repair.Cascades(items, projects), display, err
}

// itemsOf は、プロジェクト ps のアイテムを読み、画面の ID を display に足す。
func itemsOf(r port.Reader, ps []project.Project, prefix string, display map[item.ID]string) ([]item.Item, error) {
	var items []item.Item
	for _, p := range ps {
		its, err := r.Items().List(p.ID)
		if err != nil {
			return nil, err
		}
		for _, it := range its {
			display[it.ID] = it.DisplayID(prefix)
		}
		items = append(items, its...)
	}
	return items, nil
}
