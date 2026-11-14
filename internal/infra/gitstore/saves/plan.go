package saves

import (
	"cmp"
	"slices"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/infra/gitstore/filemerge"
)

// droppedRows は、合わせの片側のワークスペース Workspace のログにあって、合わせの結果では取り除かれた Target の行。
// Log はその側の Target の行、New はそのうち共通の祖先のログにない行。
type droppedRows struct {
	Workspace change.ID
	Target    change.ID
	Log       []change.Record
	New       []change.Record
}

// grouping は、ワークスペースのログの行 rows を、targets にある取り除く対象ごとに分けるまとめ方。log は、行の持ち主を
// 決めるのに読む、その版のログ全体。
type grouping func(log, rows []change.Record, targets map[string]bool) map[change.ID][]change.Record

// drop は、取り除く対象の 1 種類。targets は purged.jsonl から読んだ内部 ID、group はその行のまとめ方。
type drop struct {
	targets map[string]bool
	group   grouping
}

// droppedOf は、合わせ c の 2 つの親のそれぞれについて、c の purged.jsonl の対象のうち、c のワークスペースのログに
// 行がなく、親のログに行があるもの。対象は、スプリント・リリース(決定 C5a-8)と、完全に削除したプロジェクトの
// ルール(決定 C5b1-6。持ち主はその版のログの scope で決める)。New は、そのうち共通の祖先(1 つ目。同期の合わせと
// 同じ。base で、要るときだけ求める)のログにない行。並びは 1 つ目の親、2 つ目の親、ワークスペース・対象の内部 ID の順。
// purged.jsonl と、親ごとの違いとログは、両方の種類で 1 回だけ読む。
func droppedOf(c *object.Commit, base func() (*object.Tree, error)) ([]port.DroppedPlan, []port.DroppedRules, error) {
	merged, err := c.Tree()
	if err != nil {
		return nil, nil, err
	}
	f, err := fileIn(merged, "purged.jsonl")
	if err != nil || f == nil {
		return nil, nil, err
	}
	text, err := f.Contents()
	if err != nil {
		return nil, nil, err
	}
	drops := []drop{
		{filemerge.PurgedPlanTargets([]byte(text)), byPlanTarget},
		{filemerge.PurgedProjects([]byte(text)), byRuleOwner},
	}
	if len(drops[0].targets) == 0 && len(drops[1].targets) == 0 {
		return nil, nil, nil
	}
	ds, err := droppedBy(c, merged, base, drops)
	if err != nil {
		return nil, nil, err
	}
	var plan []port.DroppedPlan
	for _, d := range ds[0] {
		plan = append(plan, port.DroppedPlan(d))
	}
	var rules []port.DroppedRules
	for _, d := range ds[1] {
		rules = append(rules, port.DroppedRules{Workspace: d.Workspace, Project: d.Target, Log: d.Log, New: d.New})
	}
	return plan, rules, nil
}

// droppedBy は、合わせ c(木は merged)の 2 つの親のそれぞれについて、drops の種類ごとの取り除かれた行。
func droppedBy(c *object.Commit, merged *object.Tree, base func() (*object.Tree, error), drops []drop) ([][]droppedRows, error) {
	out := make([][]droppedRows, len(drops))
	for i := range 2 {
		p, err := c.Parent(i)
		if err != nil {
			return nil, err
		}
		t, err := p.Tree()
		if err != nil {
			return nil, err
		}
		ds, err := droppedFrom(t, merged, base, drops)
		if err != nil {
			return nil, err
		}
		for k := range drops {
			out[k] = append(out[k], ds[k]...)
		}
	}
	return out, nil
}

// droppedFrom は、親の版 side にあって merged にない、drops の種類ごとの対象の行。
func droppedFrom(side, merged *object.Tree, base func() (*object.Tree, error), drops []drop) ([][]droppedRows, error) {
	changes, err := object.DiffTree(side, merged)
	if err != nil {
		return nil, err
	}
	out := make([][]droppedRows, len(drops))
	for _, ch := range changes {
		ref, ok := logRef(ch.From.Name)
		if !ok || ref.Kind != change.KindWorkspace || ch.To.Name == "" {
			continue
		}
		ds, err := droppedIn(ref.ID, ch.From.Name, side, merged, base, drops)
		if err != nil {
			return nil, err
		}
		for k := range drops {
			out[k] = append(out[k], ds[k]...)
		}
	}
	for k := range out {
		slices.SortFunc(out[k], func(a, b droppedRows) int {
			return cmp.Or(strings.Compare(string(a.Workspace), string(b.Workspace)), strings.Compare(string(a.Target), string(b.Target)))
		})
	}
	return out, nil
}

// droppedIn は、ワークスペース ws のログ name について、drops の種類ごとに、merged に行がなく side に行がある
// 対象ごとの行。共通の祖先は、そのような対象があるときだけ求める。共通の祖先がなければ何も返さない。
func droppedIn(ws change.ID, name string, side, merged *object.Tree, base func() (*object.Tree, error), drops []drop) ([][]droppedRows, error) {
	sf, sideLog, err := logIn(side, name)
	if err != nil {
		return nil, err
	}
	_, mergedLog, err := logIn(merged, name)
	if err != nil {
		return nil, err
	}
	sideRows := make([]map[change.ID][]change.Record, len(drops))
	gone := make([][]change.ID, len(drops))
	found := false
	for k, d := range drops {
		sideRows[k], gone[k] = goneIn(d, sideLog, mergedLog)
		found = found || len(gone[k]) > 0
	}
	if !found {
		return make([][]droppedRows, len(drops)), nil
	}
	bt, err := base()
	if err != nil || bt == nil {
		return make([][]droppedRows, len(drops)), err
	}
	bf, err := fileIn(bt, name)
	if err != nil {
		return nil, err
	}
	fresh, err := added(bf, sf)
	if err != nil {
		return nil, err
	}
	out := make([][]droppedRows, len(drops))
	for k, d := range drops {
		newRows := d.group(sideLog, fresh, d.targets)
		for _, target := range gone[k] {
			out[k] = append(out[k], droppedRows{Workspace: ws, Target: target, Log: sideRows[k][target], New: newRows[target]})
		}
	}
	return out, nil
}

// goneIn は、d の対象ごとの、その側のログ sideLog の行と、そのうち合わせの結果のログ mergedLog に行がない対象。
func goneIn(d drop, sideLog, mergedLog []change.Record) (map[change.ID][]change.Record, []change.ID) {
	sideRows, mergedRows := d.group(sideLog, sideLog, d.targets), d.group(mergedLog, mergedLog, d.targets)
	var gone []change.ID
	for target := range sideRows {
		if _, kept := mergedRows[target]; !kept {
			gone = append(gone, target)
		}
	}
	return sideRows, gone
}

// logIn は、t の name のファイルと、その行。ファイルがなければ nil。
func logIn(t *object.Tree, name string) (*object.File, []change.Record, error) {
	f, err := fileIn(t, name)
	if err != nil {
		return nil, nil, err
	}
	rows, err := added(nil, f)
	return f, rows, err
}

// byPlanTarget は、rows のスプリント・リリースの行を、targets にある target ごとに分けたもの。
func byPlanTarget(_, rows []change.Record, targets map[string]bool) map[change.ID][]change.Record {
	out := map[change.ID][]change.Record{}
	for _, r := range rows {
		if id, ok := planning.PlanTarget(r); ok && targets[string(id)] {
			out[id] = append(out[id], r)
		}
	}
	return out
}

// hasNewPlan は、ds に、共通の祖先にない行を持つものがあるか。
func hasNewPlan(ds []port.DroppedPlan) bool {
	return slices.ContainsFunc(ds, func(d port.DroppedPlan) bool { return len(d.New) > 0 })
}

// baseOf は、合わせ c の 2 つの親の共通の祖先(1 つ目)の木を、初めて呼ばれたときに 1 回だけ求める関数。なければ nil。
func baseOf(c *object.Commit) func() (*object.Tree, error) {
	var (
		done bool
		tree *object.Tree
		err  error
	)
	return func() (*object.Tree, error) {
		if !done {
			done = true
			tree, err = baseTreeOf(c)
		}
		return tree, err
	}
}

func baseTreeOf(c *object.Commit) (*object.Tree, error) {
	ours, err := c.Parent(0)
	if err != nil {
		return nil, err
	}
	theirs, err := c.Parent(1)
	if err != nil {
		return nil, err
	}
	bases, err := ours.MergeBase(theirs)
	if err != nil || len(bases) == 0 {
		return nil, err
	}
	return bases[0].Tree()
}
