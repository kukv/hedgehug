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

// droppedPlan は、合わせ c の 2 つの親のそれぞれについて、c の purged.jsonl のスプリント・リリースの target のうち、
// c のワークスペースのログに行がなく、親のログに行があるもの(決定 C5a-8)。New は、そのうち共通の祖先(1 つ目。
// 同期の合わせと同じ。base で、要るときだけ求める)のログにない行。並びは 1 つ目の親、2 つ目の親、ワークスペース・
// target の内部 ID の順。
func droppedPlan(c *object.Commit, base func() (*object.Tree, error)) ([]port.DroppedPlan, error) {
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
	targets := filemerge.PurgedPlanTargets([]byte(text))
	if len(targets) == 0 {
		return nil, nil
	}
	var out []port.DroppedPlan
	for i := range 2 {
		p, err := c.Parent(i)
		if err != nil {
			return nil, err
		}
		t, err := p.Tree()
		if err != nil {
			return nil, err
		}
		ds, err := droppedFrom(t, merged, base, targets)
		if err != nil {
			return nil, err
		}
		out = append(out, ds...)
	}
	return out, nil
}

// droppedFrom は、親の版 side にあって merged にない、targets のスプリント・リリースの行。
func droppedFrom(side, merged *object.Tree, base func() (*object.Tree, error), targets map[string]bool) ([]port.DroppedPlan, error) {
	changes, err := object.DiffTree(side, merged)
	if err != nil {
		return nil, err
	}
	var out []port.DroppedPlan
	for _, ch := range changes {
		ref, ok := logRef(ch.From.Name)
		if !ok || ref.Kind != change.KindWorkspace || ch.To.Name == "" {
			continue
		}
		ds, err := droppedIn(ref.ID, ch.From.Name, side, merged, base, targets)
		if err != nil {
			return nil, err
		}
		out = append(out, ds...)
	}
	slices.SortFunc(out, func(a, b port.DroppedPlan) int {
		return cmp.Or(strings.Compare(string(a.Workspace), string(b.Workspace)), strings.Compare(string(a.Target), string(b.Target)))
	})
	return out, nil
}

// droppedIn は、ワークスペース ws のログ name について、merged に行がなく side に行がある target ごとの行。
// 共通の祖先は、そのような target があるときだけ求める。共通の祖先がなければ何も返さない。
func droppedIn(ws change.ID, name string, side, merged *object.Tree, base func() (*object.Tree, error), targets map[string]bool) ([]port.DroppedPlan, error) {
	sf, err := fileIn(side, name)
	if err != nil {
		return nil, err
	}
	mf, err := fileIn(merged, name)
	if err != nil {
		return nil, err
	}
	sideRows, err := planRowsIn(sf, targets)
	if err != nil {
		return nil, err
	}
	mergedRows, err := planRowsIn(mf, targets)
	if err != nil {
		return nil, err
	}
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
	newRows := byPlanTarget(fresh, targets)
	out := make([]port.DroppedPlan, 0, len(gone))
	for _, target := range gone {
		out = append(out, port.DroppedPlan{Workspace: ws, Target: target, Log: sideRows[target], New: newRows[target]})
	}
	return out, nil
}

// planRowsIn は、f のスプリント・リリースの行を、targets にある target ごとに分けたもの。f が nil なら空。
func planRowsIn(f *object.File, targets map[string]bool) (map[change.ID][]change.Record, error) {
	rows, err := added(nil, f)
	if err != nil {
		return nil, err
	}
	return byPlanTarget(rows, targets), nil
}

// byPlanTarget は、rows のスプリント・リリースの行を、targets にある target ごとに分けたもの。
func byPlanTarget(rows []change.Record, targets map[string]bool) map[change.ID][]change.Record {
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
