package saves

import (
	"maps"
	"slices"

	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

// byRuleOwner は、rows のルールの行を、持ち主のプロジェクトが targets にあるものごとに分けたもの。専用ルールの
// 持ち主は、log の scope で決める。
func byRuleOwner(log, rows []change.Record, targets map[string]bool) map[change.ID][]change.Record {
	scopes := repair.RuleScopes(log)
	out := map[change.ID][]change.Record{}
	for _, r := range rows {
		if p, ok := repair.RuleOwner(r, scopes); ok && targets[string(p)] {
			out[p] = append(out[p], r)
		}
	}
	return out
}

// hasNewRules は、ds に、共通の祖先にない行を持つものがあるか。
func hasNewRules(ds []port.DroppedRules) bool {
	return slices.ContainsFunc(ds, func(d port.DroppedRules) bool { return len(d.New) > 0 })
}

// droppedAndMoved は、合わせ c の droppedOf と movedOf。
func droppedAndMoved(c *object.Commit, base func() (*object.Tree, error)) ([]port.DroppedPlan, []port.DroppedRules, []port.MovedRules, error) {
	plan, rules, err := droppedOf(c, base)
	if err != nil {
		return nil, nil, nil, err
	}
	moved, err := movedOf(c, base)
	return plan, rules, moved, err
}

// movedOf は、合わせ c の 2 つの親のそれぞれについて、もう片側の操作で別の Store へ移したプロジェクトのルールに、
// その側で書いた、共通の祖先(base)にない行(決定 C5b1-17)。移したプロジェクトは、c の transfers.jsonl のうち、
// その親の transfers.jsonl にない entity が project の記録の target で、c にそのフォルダがないもの(別の Store への
// 移動。同じ Store の中の移動はフォルダが残る)。transfers.jsonl が c と同じ親は見ない。並びは 1 つ目の親、2 つ目の
// 親、ワークスペース・プロジェクトの内部 ID の順。
func movedOf(c *object.Commit, base func() (*object.Tree, error)) ([]port.MovedRules, error) {
	merged, err := c.Tree()
	if err != nil {
		return nil, err
	}
	mf, err := fileIn(merged, transfersFile)
	if err != nil || mf == nil {
		return nil, err
	}
	var out []port.MovedRules
	for i := range 2 {
		p, err := c.Parent(i)
		if err != nil {
			return nil, err
		}
		side, err := p.Tree()
		if err != nil {
			return nil, err
		}
		targets, err := movedAway(merged, side, mf)
		if err != nil {
			return nil, err
		}
		ds, err := freshRules(side, base, targets)
		if err != nil {
			return nil, err
		}
		out = append(out, ds...)
	}
	return out, nil
}

// transfersFile は、転送記録のファイル。
const transfersFile = "transfers.jsonl"

// recordsOf は、ファイル f の転送記録。f が nil なら nil。
func recordsOf(f *object.File) ([]transfer.Record, error) {
	if f == nil {
		return nil, nil
	}
	text, err := f.Contents()
	return transfer.ParseRecords([]byte(text)), err
}

// movedAway は、merged の転送記録(ファイル mf)のうち side の transfers.jsonl になく、merged にフォルダのない
// プロジェクトの内部 ID。side の transfers.jsonl が mf と同じなら空。
func movedAway(merged, side *object.Tree, mf *object.File) (map[string]bool, error) {
	sf, err := fileIn(side, transfersFile)
	if err != nil || (sf != nil && sf.Hash == mf.Hash) {
		return nil, err
	}
	own, err := recordsOf(sf)
	if err != nil {
		return nil, err
	}
	recs, err := recordsOf(mf)
	if err != nil {
		return nil, err
	}
	targets := map[string]bool{}
	for _, r := range recs {
		if r.Entity != transfer.EntityProject || slices.ContainsFunc(own, func(o transfer.Record) bool { return o.ID == r.ID }) {
			continue
		}
		f, err := fileIn(merged, Dirs[change.KindProject]+"/"+string(r.Target)+"/log.jsonl")
		if err != nil {
			return nil, err
		}
		if f == nil {
			targets[string(r.Target)] = true
		}
	}
	return targets, nil
}

// freshRules は、親の版 side のワークスペースのログのうち、共通の祖先にない行で、持ち主が targets にあるルールの行を、
// ワークスペースとプロジェクトごとにまとめたもの。targets が空か、共通の祖先がなければ何も返さない。
func freshRules(side *object.Tree, base func() (*object.Tree, error), targets map[string]bool) ([]port.MovedRules, error) {
	if len(targets) == 0 {
		return nil, nil
	}
	bt, err := base()
	if err != nil || bt == nil {
		return nil, err
	}
	changes, err := object.DiffTree(bt, side)
	if err != nil {
		return nil, err
	}
	var out []port.MovedRules
	for _, ch := range changes {
		ref, ok := logRef(ch.To.Name)
		if !ok || ref.Kind != change.KindWorkspace {
			continue
		}
		groups, err := freshIn(bt, side, ch.To.Name, targets)
		if err != nil {
			return nil, err
		}
		for _, p := range slices.Sorted(maps.Keys(groups)) {
			out = append(out, port.MovedRules{Workspace: ref.ID, Project: p, New: groups[p]})
		}
	}
	return out, nil
}

// freshIn は、ワークスペースのログ name の、side の行のうち base にない行で、持ち主が targets にあるルールの行を、
// プロジェクトごとにまとめたもの。
func freshIn(base, side *object.Tree, name string, targets map[string]bool) (map[change.ID][]change.Record, error) {
	bf, err := fileIn(base, name)
	if err != nil {
		return nil, err
	}
	sf, sideLog, err := logIn(side, name)
	if err != nil {
		return nil, err
	}
	fresh, err := added(bf, sf)
	return byRuleOwner(sideLog, fresh, targets), err
}
