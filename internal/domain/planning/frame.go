package planning

import (
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// FrameEnd は、移動先の枠の末尾の rank(枠のアイテムの最大の rank。F1-11)。空の枠は空文字。
// sprint が空ならバックログの枠(使えるスプリントに入っていない、未着手か進行中の作業項目)。
// スプリントの枠は、そのスプリントの削除済みでない作業項目(分類を問わない)。
func FrameEnd(its []item.Item, ws workspace.ID, sprint change.ID, plan Plan, st workspace.Settings) string {
	end := ""
	for _, it := range its {
		if counted(it, ws) && inFrame(it, sprint, plan, st) && it.Rank > end {
			end = it.Rank
		}
	}
	return end
}

func inFrame(it item.Item, sprint change.ID, plan Plan, st workspace.Settings) bool {
	if sprint != "" {
		return it.Sprint == sprint
	}
	if _, ok := plan.UsableSprint(it.Sprint); ok {
		return false
	}
	c := Category(st, it.Status)
	return c == workspace.CategoryTodo || c == workspace.CategoryDoing
}
