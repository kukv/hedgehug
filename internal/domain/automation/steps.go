package automation

import (
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// StepsDone は、1 回の操作で、親の Step がすべて完了・中止になったか(03「Step がすべて完了したとき」)。
// steps は親のすべての Step(操作のあとの値。自動化の変更は含めない。B4-8)、before は操作で変えた Step の
// 変更前のステータス(B4-9)。
func StepsDone(s workspace.Settings, steps []item.Item, before map[item.ID]change.ID) bool {
	return closed(s, steps, nil) && !closed(s, steps, before)
}

// closed は、Step が 1 つ以上あり、すべて完了・中止の分類か。override は値を差し替える Step。
func closed(s workspace.Settings, steps []item.Item, override map[item.ID]change.ID) bool {
	if len(steps) == 0 {
		return false
	}
	for _, st := range steps {
		v := st.Status
		if o, ok := override[st.ID]; ok {
			v = o
		}
		c := category(s, v)
		if c != workspace.CategoryDone && c != workspace.CategoryCanceled {
			return false
		}
	}
	return true
}

func category(s workspace.Settings, id change.ID) workspace.Category {
	for _, v := range s.Statuses {
		if v.ID == id {
			return v.Category
		}
	}
	return ""
}
