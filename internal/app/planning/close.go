package planning

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// CloseSprint は、実行中のスプリント id を完了にする(F1-12)。1 回の保存で、確定値を記録し、
// 未完了のアイテムを移動先 dest(nil ならバックログ)の枠の末尾へ、いまの並び順のまま移す。
// id が実行中でないか、dest が計画中のスプリントでなければ ErrNotFound。
func (s Service) CloseSprint(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID, dest *change.ID) error {
	return s.writePlan(ctx, store, ws, revival{workspace: true},
		func(p planning.Plan) (string, error) {
			sp, err := closable(p, id, dest)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("スプリント「%s」を完了", oneLine(sp.Name)), nil
		},
		func(tx port.Tx, p planning.Plan) error {
			old, err := closable(p, id, dest)
			if err != nil {
				return err
			}
			st, err := tx.Workspaces().Settings(ws)
			if err != nil {
				return err
			}
			all, err := works(tx, ws)
			if err != nil {
				return err
			}
			// 確定値は、アイテムを移す前の値で求める。
			done := planning.Confirm(all, ws, old, st, s.Clock.Now())
			its := make([]item.Item, 0, len(all))
			for _, w := range all {
				its = append(its, w.Item)
			}
			if err := moveUndone(tx, its, ws, old.ID, dest, p, st); err != nil {
				return err
			}
			sp := old
			sp.State = planning.Closed
			return tx.WriteWorkspaces().Append(ws, append([]change.Draft{planning.CloseDraft(id, done)}, planning.SprintDrafts(old, sp)...)...)
		})
}

// closable は、完了できるスプリント id。dest は nil か計画中のスプリントでなければならない。
func closable(p planning.Plan, id change.ID, dest *change.ID) (planning.Sprint, error) {
	sp, ok := p.UsableSprint(id)
	if !ok || sp.State != planning.Active {
		return planning.Sprint{}, port.ErrNotFound
	}
	if dest != nil {
		if d, ok := p.UsableSprint(*dest); !ok || d.State != planning.Planned {
			return planning.Sprint{}, port.ErrNotFound
		}
	}
	return sp, nil
}

// moveUndone は、スプリント id の未完了(分類が未着手か進行中、削除済みを除く)を、いまの並び順のまま
// 移動先の枠の末尾へ移す。枠が空なら rank は変えない(F1-11)。
func moveUndone(tx port.Tx, all []item.Item, ws workspace.ID, id change.ID, dest *change.ID,
	p planning.Plan, st workspace.Settings,
) error {
	var to any // バックログは null
	var frame change.ID
	if dest != nil {
		to, frame = *dest, *dest
	}
	end := planning.FrameEnd(all, ws, frame, p, st)
	for _, it := range undone(all, ws, id, st) {
		ds := []change.Draft{change.NewSet(item.SprintField, id, to)}
		if end != "" {
			end = rank.After(end)
			ds = append(ds, change.NewSet("rank", it.Rank, end))
		}
		if err := tx.WriteItems().Append(it.ID, ds...); err != nil {
			return err
		}
	}
	return nil
}

// undone は、スプリント id に入っている未完了の作業項目を、ワークスペースの並び順で返す。
func undone(all []item.Item, ws workspace.ID, id change.ID, st workspace.Settings) []item.Item {
	var out []item.Item
	for _, it := range all {
		if it.Deleted != nil || it.Place.Workspace != ws || !planning.IsWork(it.Kind) || it.Sprint != id {
			continue
		}
		if c := planning.Category(st, it.Status); c == workspace.CategoryTodo || c == workspace.CategoryDoing {
			out = append(out, it)
		}
	}
	item.SortSiblings(out)
	return out
}
