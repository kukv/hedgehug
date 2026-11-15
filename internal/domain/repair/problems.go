package repair

import (
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Problem は、アイテムに見つかるデータの問題 1 つ(08「検出する項目」のアイテムを指す行)。
type Problem string

// 問題の種類。ItemProblems が返す並びは、この宣言の順。
const (
	ProblemNoParent         Problem = "no-parent"         // 親がない
	ProblemBadParentKind    Problem = "bad-parent-kind"   // 種類に合わない親子
	ProblemParentCycle      Problem = "parent-cycle"      // 親の循環
	ProblemNoStatus         Problem = "no-status"         // ステータスがない
	ProblemMissingValue     Problem = "missing-value"     // 存在しない値
	ProblemMissingPerson    Problem = "missing-person"    // 存在しない人
	ProblemMissingPlan      Problem = "missing-plan"      // 存在しないスプリント・リリース
	ProblemDanglingRelation Problem = "dangling-relation" // 宙づりの関連
	ProblemParentElsewhere  Problem = "parent-elsewhere"  // 親が別のワークスペース・プロジェクトにある
	ProblemDateOrder        Problem = "date-order"        // 日付の逆転
	ProblemNumberOverlap    Problem = "number-overlap"    // 番号の重なり
	ProblemIssueTwice       Problem = "issue-twice"       // Issue の二重リンク
)

// StoreState は、診断が読む Store の状態。
type StoreState struct {
	Items    []item.Item // 削除済みを含む Store のすべて
	Settings map[workspace.ID]workspace.Settings
	Plans    map[workspace.ID]planning.Plan // 削除済みを含む
	Members  []member.Member
	Issues   map[item.ID][]string // アイテムがリンクしている Issue の鍵(itemlink の Ref.SameKey)
}

// ItemProblems は、削除されていないアイテムごとの問題を返す。問題のないアイテムは入れない。
// 1 つのアイテムの問題は Problem の宣言の順。検索の is:invalid と診断が、同じ答えを使う(決定 E1-7)。
func ItemProblems(s StoreState) map[item.ID][]Problem {
	tree := newTreeCheck(s.Items)
	overlap := overlappingNumbers(s.Items)
	twice := issueTwice(s)
	values := newValueCheck(s)
	out := map[item.ID][]Problem{}
	for _, it := range item.Live(s.Items) {
		var ps []Problem
		ps = tree.problems(it, ps)
		ps = values.problems(it, ps)
		if overlap[numberKey{it.Place.Workspace, it.Place.Number}] {
			ps = append(ps, ProblemNumberOverlap)
		}
		if twice[it.ID] {
			ps = append(ps, ProblemIssueTwice)
		}
		if len(ps) > 0 {
			orderProblems(ps)
			out[it.ID] = ps
		}
	}
	return out
}
