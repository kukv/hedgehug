package repair

import (
	"cmp"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
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

// Stray は、親とワークスペースが違う子 1 つと、移す先(親のワークスペースとプロジェクト)。
type Stray struct {
	Child     item.ID
	Workspace workspace.ID
	Project   project.ID
}

// WorkspaceMismatch は、削除されていない子と削除されていない親でワークスペースが違うもののうち、
// 深さの浅い順、旧い番号の小さい順、内部 ID の順の最初の 1 件(決定 C3b-2)。なければ false。
func WorkspaceMismatch(items []item.Item) (Stray, bool) {
	byID := make(map[item.ID]item.Item, len(items))
	for _, it := range items {
		byID[it.ID] = it
	}
	type found struct {
		it    item.Item
		to    Stray
		depth int
	}
	var all []found
	for _, it := range item.Live(items) {
		if it.Place.Parent == nil {
			continue
		}
		p, ok := byID[*it.Place.Parent]
		if !ok || p.Deleted != nil || p.Place.Workspace == it.Place.Workspace {
			continue
		}
		all = append(all, found{it, Stray{it.ID, p.Place.Workspace, p.Place.Project}, depthOf(it, byID)})
	}
	if len(all) == 0 {
		return Stray{}, false
	}
	best := slices.MinFunc(all, func(a, b found) int {
		return cmp.Or(
			cmp.Compare(a.depth, b.depth),
			cmp.Compare(a.it.Place.Number, b.it.Place.Number),
			strings.Compare(string(a.it.ID), string(b.it.ID)),
		)
	})
	return best.to, true
}

// DeletedBelow は、child の下で、削除済みのものだけをたどって届く削除済みのもの(決定 C3b-1)。
// 並びは問わない。削除されていない子孫は含めない(それは自分で直す子になる)。同じものは 2 度たどらない。
func DeletedBelow(child item.ID, items []item.Item) []item.Item {
	kids := make(map[item.ID][]item.Item)
	for _, it := range items {
		if it.Deleted != nil && it.Place.Parent != nil {
			kids[*it.Place.Parent] = append(kids[*it.Place.Parent], it)
		}
	}
	seen := map[item.ID]bool{child: true}
	var out []item.Item
	queue := []item.ID{child}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, k := range kids[cur] {
			if seen[k.ID] {
				continue
			}
			seen[k.ID] = true
			out = append(out, k)
			queue = append(queue, k.ID)
		}
	}
	return out
}

// NumberByOld は、child に next、続けて deleted を旧い番号の小さい順(同じなら内部 ID の順)に 1 つずつ振った番号(決定 C3b-3)。
// deleted は変えない。
func NumberByOld(child item.Item, deleted []item.Item, next int) map[item.ID]int {
	sorted := slices.Clone(deleted)
	slices.SortFunc(sorted, func(a, b item.Item) int {
		return cmp.Or(cmp.Compare(a.Place.Number, b.Place.Number), strings.Compare(string(a.ID), string(b.ID)))
	})
	nums := map[item.ID]int{child.ID: next}
	for i, it := range sorted {
		nums[it.ID] = next + 1 + i
	}
	return nums
}
