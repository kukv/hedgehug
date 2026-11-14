package saves

import (
	"slices"

	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/repair"
)

// sidesOf は、restore の行を書いた同期の修復の保存 c の Sides(決定 C4b2-9)。c の前を、ほかの修復の保存を通って
// 辿ったいちばん近い合わせで、c が書いたエンティティ es のログの、共通の祖先(1 つ目。同期の合わせと同じ)にない
// 行の ID を、1 つ目の親の側(Ours)と 2 つ目の親の側(Theirs)に分ける。restore の行がないか、合わせに届かなければ空。
func sidesOf(c *object.Commit, es []port.EntityRows) (repair.Facts, error) {
	if !slices.ContainsFunc(es, func(e port.EntityRows) bool {
		return slices.ContainsFunc(e.Rows, func(r change.Record) bool { return r.T == change.Restore })
	}) {
		return repair.Facts{}, nil
	}
	m, err := nearestMerge(c)
	if err != nil || m == nil {
		return repair.Facts{}, err
	}
	ours, err := m.Parent(0)
	if err != nil {
		return repair.Facts{}, err
	}
	theirs, err := m.Parent(1)
	if err != nil {
		return repair.Facts{}, err
	}
	bases, err := ours.MergeBase(theirs)
	if err != nil || len(bases) == 0 {
		return repair.Facts{}, err
	}
	f := repair.Facts{Ours: map[change.ID]bool{}, Theirs: map[change.ID]bool{}}
	for _, side := range []struct {
		c   *object.Commit
		out map[change.ID]bool
	}{{ours, f.Ours}, {theirs, f.Theirs}} {
		if err := addSide(side.out, bases[0], side.c, es); err != nil {
			return repair.Facts{}, err
		}
	}
	return f, nil
}

// nearestMerge は、c から最初の親を、同期の修復の保存を通って辿った、いちばん近い合わせ。ほかの保存か最初の保存に
// 当たれば nil。
func nearestMerge(c *object.Commit) (*object.Commit, error) {
	for c.NumParents() > 0 {
		p, err := c.Parent(0)
		if err != nil {
			return nil, err
		}
		if p.NumParents() == 2 {
			return p, nil
		}
		es, err := entities(p)
		if err != nil || !isRepair(p, es) {
			return nil, err
		}
		c = p
	}
	return nil, nil
}

// addSide は、es のエンティティのログについて、side の版にあって base の版にない行の ID を out に足す。
func addSide(out map[change.ID]bool, base, side *object.Commit, es []port.EntityRows) error {
	bt, err := base.Tree()
	if err != nil {
		return err
	}
	st, err := side.Tree()
	if err != nil {
		return err
	}
	for _, e := range es {
		name := logPath(e.Ref)
		old, err := fileIn(bt, name)
		if err != nil {
			return err
		}
		f, err := fileIn(st, name)
		if err != nil {
			return err
		}
		rows, err := added(old, f)
		if err != nil {
			return err
		}
		for _, r := range rows {
			out[r.ID] = true
		}
	}
	return nil
}

// logPath は、エンティティ ref の log.jsonl のパス(メンバーはフォルダの直下)。
func logPath(ref change.Ref) string {
	if ref.Kind == change.KindMembers {
		return Dirs[ref.Kind] + "/log.jsonl"
	}
	return Dirs[ref.Kind] + "/" + string(ref.ID) + "/log.jsonl"
}
