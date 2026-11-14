package repair

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// valueCheck は、値・人・計画・関連・日付の問題の判定。調べるために引く集合を先に作っておく。
type valueCheck struct {
	settings map[workspace.ID]workspace.Settings
	members  map[string]bool // 小文字にしたログイン名
	sprints  map[workspace.ID]map[change.ID]bool
	releases map[workspace.ID]map[change.ID]bool
	exists   map[item.ID]bool // 削除済みも含む
}

func newValueCheck(s StoreState) valueCheck {
	v := valueCheck{
		settings: s.Settings,
		members:  map[string]bool{},
		sprints:  map[workspace.ID]map[change.ID]bool{},
		releases: map[workspace.ID]map[change.ID]bool{},
		exists:   make(map[item.ID]bool, len(s.Items)),
	}
	for _, m := range s.Members {
		v.members[strings.ToLower(m.Login)] = true
	}
	for ws, p := range s.Plans {
		v.sprints[ws], v.releases[ws] = map[change.ID]bool{}, map[change.ID]bool{}
		for _, sp := range p.Sprints {
			v.sprints[ws][sp.ID] = true
		}
		for _, r := range p.Releases {
			v.releases[ws][r.ID] = true
		}
	}
	for _, it := range s.Items {
		v.exists[it.ID] = true
	}
	return v
}

func hasValue(list []workspace.Value, id change.ID) bool {
	return slices.ContainsFunc(list, func(x workspace.Value) bool { return x.ID == id })
}

// problems は、削除されていない it の、ステータス・値・人・計画・関連・日付の問題を ps に足す。
// ワークスペースの設定が分からないアイテムは、ステータス・値・人の判定をしない。
func (v valueCheck) problems(it item.Item, ps []Problem) []Problem {
	set, known := v.settings[it.Place.Workspace]
	if known && !hasValue(set.Statuses, it.Status) {
		ps = append(ps, ProblemNoStatus)
	}
	if known && missingValue(it, set) {
		ps = append(ps, ProblemMissingValue)
	}
	if known && v.missingPerson(it, set) {
		ps = append(ps, ProblemMissingPerson)
	}
	if v.missingPlan(it) {
		ps = append(ps, ProblemMissingPlan)
	}
	if slices.ContainsFunc(it.Relations, func(r item.Relation) bool { return !v.exists[r.To] }) {
		ps = append(ps, ProblemDanglingRelation)
	}
	if it.Start != "" && it.End != "" && it.End < it.Start {
		ps = append(ps, ProblemDateOrder)
	}
	return ps
}

// missingValue は、優先度・関連システム・選択式の値が、ワークスペースの一覧にないか。空の優先度は値でない。
func missingValue(it item.Item, set workspace.Settings) bool {
	if it.Priority != "" && !hasValue(set.Priorities, it.Priority) {
		return true
	}
	if slices.ContainsFunc(it.Systems, func(id change.ID) bool { return !hasValue(set.Systems, id) }) {
		return true
	}
	for propID, ids := range it.Choices {
		i := slices.IndexFunc(set.Properties, func(p workspace.Property) bool { return p.ID == propID })
		// プロパティが消えていれば、その値はすべて存在しない。
		if slices.ContainsFunc(ids, func(id change.ID) bool { return i < 0 || !hasValue(set.Properties[i].Choices, id) }) {
			return true
		}
	}
	return false
}

// missingPerson は、担当者と、ユーザー型の追加プロパティのログイン名が、メンバーにいないか。
func (v valueCheck) missingPerson(it item.Item, set workspace.Settings) bool {
	if it.Assignee != "" && !v.members[strings.ToLower(it.Assignee)] {
		return true
	}
	for _, p := range set.Properties {
		if p.Type != workspace.TypeUser || p.Builtin {
			continue
		}
		var login string
		if json.Unmarshal(it.Fields[p.ID], &login) == nil && login != "" && !v.members[strings.ToLower(login)] {
			return true
		}
	}
	return false
}

// missingPlan は、スプリントかリリースが、ワークスペースの計画(削除済みを含む)にないか。
// 呼ぶ側は、読めたすべてのワークスペースの計画を Plans に入れる(設定と違い、ログから作るので読めないことがない)。
// なので Plans にワークスペースがないのは、そのワークスペースがないときで、スプリント・リリースも本当にない。
func (v valueCheck) missingPlan(it item.Item) bool {
	ws := it.Place.Workspace
	return (it.Sprint != "" && !v.sprints[ws][it.Sprint]) ||
		(it.Release != "" && !v.releases[ws][it.Release])
}

// problemOrder は Problem の宣言の順。
var problemOrder = []Problem{
	ProblemNoParent, ProblemBadParentKind, ProblemParentCycle, ProblemNoStatus, ProblemMissingValue,
	ProblemMissingPerson, ProblemMissingPlan, ProblemDanglingRelation, ProblemParentElsewhere,
	ProblemDateOrder, ProblemNumberOverlap, ProblemIssueTwice,
}

// orderProblems は、ps を Problem の宣言の順にそろえる。
func orderProblems(ps []Problem) {
	slices.SortStableFunc(ps, func(a, b Problem) int {
		return slices.Index(problemOrder, a) - slices.Index(problemOrder, b)
	})
}
