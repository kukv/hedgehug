package repair

import (
	"cmp"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// Mismatch は、親とプロジェクトが違う子 1 つと、移す先(親のプロジェクト)。
type Mismatch struct {
	Child   item.ID
	Project project.ID
}

// ProjectMismatch は、削除されていない子と削除されていない親が同じワークスペースで、プロジェクトだけが違うものの
// うち、深さの浅い順、同じなら内部 ID の順の最初の 1 件(保存形式「同期の修復」の (5)。決定 C3a-16)。
// 深さは、親をたどった数(輪になっていても、同じものは 2 度数えない)。ワークスペースが違う子は返さない。
func ProjectMismatch(items []item.Item) (Mismatch, bool) {
	byID := make(map[item.ID]item.Item, len(items))
	for _, it := range items {
		byID[it.ID] = it
	}
	type found struct {
		m     Mismatch
		depth int
	}
	var all []found
	for _, it := range item.Live(items) {
		if it.Place.Parent == nil {
			continue
		}
		p, ok := byID[*it.Place.Parent]
		if !ok || p.Deleted != nil || p.Place.Workspace != it.Place.Workspace || p.Place.Project == it.Place.Project {
			continue
		}
		all = append(all, found{Mismatch{it.ID, p.Place.Project}, depthOf(it, byID)})
	}
	if len(all) == 0 {
		return Mismatch{}, false
	}
	best := slices.MinFunc(all, func(a, b found) int {
		return cmp.Or(cmp.Compare(a.depth, b.depth), strings.Compare(string(a.m.Child), string(b.m.Child)))
	})
	return best.m, true
}

// depthOf は、it から親をたどった数。見つからないところ、または一度通ったところで止める。
func depthOf(it item.Item, byID map[item.ID]item.Item) int {
	seen := map[item.ID]bool{it.ID: true}
	n := 0
	for it.Place.Parent != nil && !seen[*it.Place.Parent] {
		p, ok := byID[*it.Place.Parent]
		if !ok {
			break
		}
		seen[p.ID] = true
		it = p
		n++
	}
	return n
}
