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
// 同期の合わせと同じ)のログにない行。並びは 1 つ目の親、2 つ目の親、ワークスペース・target の内部 ID の順。
func droppedPlan(c *object.Commit) ([]port.DroppedPlan, error) {
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
	base, err := bases[0].Tree()
	if err != nil {
		return nil, err
	}
	var out []port.DroppedPlan
	for _, p := range []*object.Commit{ours, theirs} {
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
func droppedFrom(side, merged, base *object.Tree, targets map[string]bool) ([]port.DroppedPlan, error) {
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
func droppedIn(ws change.ID, name string, side, merged, base *object.Tree, targets map[string]bool) ([]port.DroppedPlan, error) {
	var rows [3]map[change.ID][]change.Record
	for i, t := range []*object.Tree{side, merged, base} {
		f, err := fileIn(t, name)
		if err != nil {
			return nil, err
		}
		if rows[i], err = planRowsIn(f, targets); err != nil {
			return nil, err
		}
	}
	var out []port.DroppedPlan
	for target, log := range rows[0] {
		if _, kept := rows[1][target]; !kept {
			out = append(out, port.DroppedPlan{Workspace: ws, Target: target, Log: log, New: notIn(log, rows[2][target])})
		}
	}
	return out, nil
}

// notIn は、rows のうち、old に同じ ID の行がないもの。
func notIn(rows, old []change.Record) []change.Record {
	return slices.DeleteFunc(slices.Clone(rows), func(r change.Record) bool {
		return slices.ContainsFunc(old, func(o change.Record) bool { return o.ID == r.ID })
	})
}

// planRowsIn は、f のスプリント・リリースの行を、targets にある target ごとに分けたもの。f が nil なら空。
func planRowsIn(f *object.File, targets map[string]bool) (map[change.ID][]change.Record, error) {
	rows, err := added(nil, f)
	if err != nil {
		return nil, err
	}
	out := map[change.ID][]change.Record{}
	for _, r := range rows {
		if id, ok := planning.PlanTarget(r); ok && targets[string(id)] {
			out[id] = append(out[id], r)
		}
	}
	return out, nil
}

// hasNewPlan は、ds に、共通の祖先にない行を持つものがあるか。
func hasNewPlan(ds []port.DroppedPlan) bool {
	return slices.ContainsFunc(ds, func(d port.DroppedPlan) bool { return len(d.New) > 0 })
}
