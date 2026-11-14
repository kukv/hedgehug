package transfer

import (
	"cmp"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// ProjectSet は、プロジェクトを移すときの、移すもの全体。Epics は Epic ごとの集まり
// (削除されていない Epic を順位の順 → 削除済みの Epic を順位の順)。各 Set の Outside は空で、
// 外との関連は ProjectSet.Outside にまとめる(Epic ごとに数えると、ほかの Epic の下のものが外になるため)。
type ProjectSet struct {
	Project project.ID
	Epics   []Set
	Outside []Cut
}

// Items は、Epic ごとに Root → Below の順のアイテム。
func (s ProjectSet) Items() []item.Item {
	var out []item.Item
	for _, e := range s.Epics {
		out = append(out, e.Root)
		out = append(out, e.Below...)
	}
	return out
}

// Contains は、id が移すもの全体の中にいるか。
func (s ProjectSet) Contains(id item.ID) bool {
	return slices.ContainsFunc(s.Epics, func(e Set) bool { return e.Contains(id) })
}

// CollectProject は、プロジェクト p の下のすべてのアイテム(削除済みを含む)と、
// 集まりの外の(all にいる)アイテムとの関連を集める。all にいない相手への関連は数えない。
func CollectProject(p project.ID, all []item.Item) ProjectSet {
	var epics []item.Item
	for _, it := range all {
		if it.Place.Project == p && it.Place.Parent == nil {
			epics = append(epics, it)
		}
	}
	item.SortSiblings(epics)
	s := ProjectSet{Project: p}
	for _, deleted := range []bool{false, true} {
		for _, e := range epics {
			if (e.Deleted != nil) != deleted {
				continue
			}
			set, err := Collect(e.ID, all)
			if err != nil { // 親のない Step は Collect が断る。移せない形なので、下のものも含めて集めない
				continue
			}
			set.Outside = nil
			order := Number(set.Root, set.Below, 0)
			slices.SortFunc(set.Below, func(a, b item.Item) int { return cmp.Compare(order[a.ID], order[b.ID]) })
			s.Epics = append(s.Epics, set)
		}
	}
	s.Outside = cutsOutside(s.Contains, all)
	return s
}

// cutsOutside は、contains が真のもの全体と、外の(all にいる)アイテムとの関連を、両方の側から数える
// (Collect と同じ並びと重複の除き方)。ProjectSet と WorkspaceSet が共有する。
func cutsOutside(contains func(item.ID) bool, all []item.Item) []Cut {
	present := map[item.ID]bool{}
	for _, it := range all {
		present[it.ID] = true
	}
	var out []Cut
	for _, it := range all {
		for _, rel := range it.Relations {
			if present[rel.To] && contains(it.ID) != contains(rel.To) {
				out = append(out, Cut{From: it.ID, To: rel.To, Type: rel.Type})
			}
		}
	}
	slices.SortFunc(out, func(a, b Cut) int {
		return cmp.Or(strings.Compare(string(a.From), string(b.From)), strings.Compare(string(a.To), string(b.To)), strings.Compare(a.Type, b.Type))
	})
	return slices.Compact(out)
}

// NumberProject は、移動先のワークスペースでの番号を決める。Epic ごとに Number と同じ規則で、
// 使った数だけ next を進めて続ける。
func NumberProject(s ProjectSet, next int) map[item.ID]int {
	out := map[item.ID]int{}
	for _, e := range s.Epics {
		for id, n := range Number(e.Root, e.Below, next) {
			out[id] = n
		}
		// Below は Descendants で集めた、Root から辿れる重複のないものだけ。Number は Root と Below の全員に 1 つずつ振る。
		next += 1 + len(e.Below)
	}
	return out
}
