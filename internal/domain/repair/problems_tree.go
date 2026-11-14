package repair

import (
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// treeCheck は、親子の問題(親がない・種類に合わない親子・親の循環・親が別の場所にある)の判定。
type treeCheck struct {
	byID   map[item.ID]item.Item
	inRing map[item.ID]bool
}

func newTreeCheck(items []item.Item) treeCheck {
	t := treeCheck{byID: make(map[item.ID]item.Item, len(items)), inRing: map[item.ID]bool{}}
	for _, it := range items {
		t.byID[it.ID] = it
	}
	for _, ring := range Rings(items) {
		for _, id := range ring {
			t.inRing[id] = true
		}
	}
	return t
}

// problems は、削除されていない it の親子の問題を、Problem の宣言の順で ps に足す。
func (t treeCheck) problems(it item.Item, ps []Problem) []Problem {
	parent, found := item.Item{}, false
	if it.Place.Parent != nil {
		parent, found = t.byID[*it.Place.Parent]
	}
	live := found && parent.Deleted == nil
	// 親がない: Epic 以外で、親がないか、削除済みか、Store にない。
	if it.Kind != item.Epic && !live {
		ps = append(ps, ProblemNoParent)
	}
	// 種類に合わない親子: 親がいて置けない。Epic は親を持つだけで合わない(親が Store になくても)。
	switch {
	case it.Kind == item.Epic && it.Place.Parent != nil:
		ps = append(ps, ProblemBadParentKind)
	case live && !item.CanPlace(it.Kind, &parent.Kind):
		ps = append(ps, ProblemBadParentKind)
	}
	if t.inRing[it.ID] {
		ps = append(ps, ProblemParentCycle)
	}
	if live && (parent.Place.Workspace != it.Place.Workspace || parent.Place.Project != it.Place.Project) {
		ps = append(ps, ProblemParentElsewhere)
	}
	return ps
}

type numberKey struct {
	workspace workspace.ID
	number    int
}

// overlappingNumbers は、同じワークスペースで 2 件以上のアイテムが持つ番号。削除済みも数える。
func overlappingNumbers(items []item.Item) map[numberKey]bool {
	count := map[numberKey]int{}
	for _, it := range items {
		count[numberKey{it.Place.Workspace, it.Place.Number}]++
	}
	out := map[numberKey]bool{}
	for k, n := range count {
		if n >= 2 {
			out[k] = true
		}
	}
	return out
}

// issueTwice は、Issue の二重リンクに当たる削除されていないアイテム。
// 1 つの Issue に 2 件以上の削除されていないアイテムがリンクしているとそのすべて、
// 1 件に 2 つ以上の Issue がリンクしているとその 1 件。同じ鍵の重複は 1 つと数える。
func issueTwice(s StoreState) map[item.ID]bool {
	byIssue := map[string][]item.ID{}
	out := map[item.ID]bool{}
	for _, it := range item.Live(s.Items) {
		keys := map[string]bool{}
		for _, k := range s.Issues[it.ID] {
			if !keys[k] {
				keys[k] = true
				byIssue[k] = append(byIssue[k], it.ID)
			}
		}
		if len(keys) >= 2 {
			out[it.ID] = true
		}
	}
	for _, ids := range byIssue {
		if len(ids) >= 2 {
			for _, id := range ids {
				out[id] = true
			}
		}
	}
	return out
}
