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

// droppedPlan は、合わせ c の 2 つの親のそれぞれについて、c の purged.jsonl のスプリント・リリースの target のうち、
// c のワークスペースのログに行がなく、親のログに行があるもの(決定 C5a-8)。New は、そのうち共通の祖先(1 つ目。
// 同期の合わせと同じ。base で、要るときだけ求める)のログにない行。並びは 1 つ目の親、2 つ目の親、ワークスペース・
// target の内部 ID の順。
func droppedPlan(c *object.Commit, base func() (*object.Tree, error)) ([]port.DroppedPlan, error) {
	ds, err := droppedBy(c, base, filemerge.PurgedPlanTargets, byPlanTarget)
	if err != nil {
		return nil, err
	}
	var out []port.DroppedPlan
	for _, d := range ds {
		out = append(out, port.DroppedPlan(d))
	}
	return out, nil
}

// droppedBy は、合わせ c の 2 つの親のそれぞれについて、c の purged.jsonl から targetsOf で読んだ対象のうち、
// c のワークスペースのログに行がなく、親のログに行があるもの。行は group で対象ごとに分ける。並びは droppedPlan と同じ。
func droppedBy(c *object.Commit, base func() (*object.Tree, error), targetsOf func([]byte) map[string]bool, group grouping) ([]droppedRows, error) {
	merged, err := c.Tree()
	if err != nil {
		return nil, err
	}
	f, err := fileIn(merged, "purged.jsonl")
	if err != nil || f == nil {
		return nil, err
	}
	text, err := f.Contents()
	if err != nil {
		return nil, err
	}
	targets := targetsOf([]byte(text))
	if len(targets) == 0 {
		return nil, nil
	}
	var out []droppedRows
	for i := range 2 {
		p, err := c.Parent(i)
		if err != nil {
			return nil, err
		}
		t, err := p.Tree()
		if err != nil {
			return nil, err
		}
		ds, err := droppedFrom(t, merged, base, targets, group)
		if err != nil {
			return nil, err
		}
		out = append(out, ds...)
	}
	return out, nil
}

// droppedFrom は、親の版 side にあって merged にない、targets の対象の行。
func droppedFrom(side, merged *object.Tree, base func() (*object.Tree, error), targets map[string]bool, group grouping) ([]droppedRows, error) {
	changes, err := object.DiffTree(side, merged)
	if err != nil {
		return nil, err
	}
	var out []droppedRows
	for _, ch := range changes {
		ref, ok := logRef(ch.From.Name)
		if !ok || ref.Kind != change.KindWorkspace || ch.To.Name == "" {
			continue
		}
		ds, err := droppedIn(ref.ID, ch.From.Name, side, merged, base, targets, group)
		if err != nil {
			return nil, err
		}
		out = append(out, ds...)
	}
	slices.SortFunc(out, func(a, b droppedRows) int {
		return cmp.Or(strings.Compare(string(a.Workspace), string(b.Workspace)), strings.Compare(string(a.Target), string(b.Target)))
	})
	return out, nil
}

// droppedIn は、ワークスペース ws のログ name について、merged に行がなく side に行がある対象ごとの行。
// 共通の祖先は、そのような対象があるときだけ求める。共通の祖先がなければ何も返さない。
func droppedIn(ws change.ID, name string, side, merged *object.Tree, base func() (*object.Tree, error), targets map[string]bool, group grouping) ([]droppedRows, error) {
	sf, sideLog, err := logIn(side, name)
	if err != nil {
		return nil, err
	}
	_, mergedLog, err := logIn(merged, name)
	if err != nil {
		return nil, err
	}
	sideRows, mergedRows := group(sideLog, sideLog, targets), group(mergedLog, mergedLog, targets)
	var gone []change.ID
	for target := range sideRows {
		if _, kept := mergedRows[target]; !kept {
			gone = append(gone, target)
		}
	}
	if len(gone) == 0 {
		return nil, nil
	}
	bt, err := base()
	if err != nil || bt == nil {
		return nil, err
	}
	bf, err := fileIn(bt, name)
	if err != nil {
		return nil, err
	}
	fresh, err := added(bf, sf)
	if err != nil {
		return nil, err
	}
	newRows := group(sideLog, fresh, targets)
	out := make([]droppedRows, 0, len(gone))
	for _, target := range gone {
		out = append(out, droppedRows{Workspace: ws, Target: target, Log: sideRows[target], New: newRows[target]})
	}
	return out, nil
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
