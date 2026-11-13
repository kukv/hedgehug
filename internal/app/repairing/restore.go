package repairing

import (
	"context"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// placeField は、アイテムの場所の項目名。
const placeField = "place"

// found は、取り消す削除 1 つと、それを見つけたログのエンティティ(snapshot.entities の位置)。
type found struct {
	repair.Overlap
	at int
}

// restoreOverlaps は、削除と変更が重なったものの削除を取り消す(決定 C3a-9)。
// 削除の op ごとに 1 回保存し、その op の行があるログごとに change.Undo の行を書く。
func restoreOverlaps(ctx context.Context, w port.RepairWriter) error {
	s, err := readSnapshot(w)
	if err != nil {
		return err
	}
	for _, f := range overlaps(s, w.Facts()) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := undo(w, s, f); err != nil {
			return err
		}
	}
	return nil
}

// overlaps は、すべてのログの重なりを、削除の op の辞書順に返す。同じ op は、最初に見つけたログのもの 1 つ。
func overlaps(s snapshot, facts repair.Facts) []found {
	in := newIncidentals(s)
	byOp := map[change.ID]found{}
	for i, e := range s.entities {
		for _, o := range repair.Overlaps(e.log, facts, in.of(e.ref)) {
			if _, ok := byOp[o.Op]; !ok {
				byOp[o.Op] = found{Overlap: o, at: i}
			}
		}
	}
	out := make([]found, 0, len(byOp))
	for _, f := range byOp {
		out = append(out, f)
	}
	slices.SortFunc(out, func(a, b found) int { return strings.Compare(string(a.Op), string(b.Op)) })
	return out
}

// undo は、削除 f の op の行を、1 回の保存で取り消す。戻すかは、保存の中で読み直したログで決める
// (前の取り消しで先端が変わっていることがある)。
func undo(w port.RepairWriter, s snapshot, f found) error {
	op := port.Operation{Actor: syncActor, Summary: summary(s, f)}
	return w.Write(op, func(tx port.Tx) error {
		for _, e := range s.entities {
			if !slices.ContainsFunc(e.log, func(r change.Record) bool { return r.Op == f.Op }) {
				continue
			}
			log, err := tx.Changes().Of(e.ref)
			if err != nil {
				return err
			}
			if err := appendTo(tx, e.ref, change.Undo(log, f.Op)...); err != nil {
				return err
			}
		}
		return nil
	})
}

// summary は、取り消しのアクティビティの文(計画の「アクティビティの文」)。名前は、改行と続いた空白を
// 1 つの空白にする(保存の Summary は 1 行だけ書ける。名前は合わせたログから読むので、ここでそろえる)。
func summary(s snapshot, f found) string {
	e := s.entities[f.at]
	if f.F == change.DeletedField {
		return e.display + " の削除を取り消し"
	}
	name := strings.Join(strings.Fields(workspace.NameOfDeleted(e.log, f.F)), " ")
	return "「" + name + "」の削除を取り消し"
}

// incidentals は、ほかのものへの操作で一緒に書いた行を見分ける材料(決定 C3a-9)。
type incidentals struct {
	deleteOps map[change.ID]bool             // ワークスペースのログに delete の行を書いた op
	placed    map[change.ID]map[item.ID]bool // op ごとの、set place を書いたアイテム
	items     map[item.ID]item.Item          // 祖先をたどるためのアイテム
}

func newIncidentals(s snapshot) incidentals {
	in := incidentals{deleteOps: map[change.ID]bool{}, placed: map[change.ID]map[item.ID]bool{}, items: s.items}
	for _, e := range s.entities {
		for _, r := range e.log {
			switch {
			case e.ref.Kind == change.KindWorkspace && r.T == change.Delete:
				in.deleteOps[r.Op] = true
			case e.ref.Kind == change.KindItem && r.T == change.Set && r.F == placeField:
				if in.placed[r.Op] == nil {
					in.placed[r.Op] = map[item.ID]bool{}
				}
				in.placed[r.Op][e.ref.ID] = true
			}
		}
	}
	return in
}

// of は、ref のログの行が、ほかのものへの操作で一緒に書いた行かを答える関数。
// 値・プロパティの削除でほかのエンティティに書いた置き換え(アイテム・プロジェクトのログの行で、同じ op で
// ワークスペースのログに delete の行がある)と、親の移動で下のものに書いた場所(同じ op で祖先のアイテムにも
// set place がある)。ワークスペースのログの行はこの op の見方では数えない(テンプレートの保存は、名前の変更と
// 子の削除を 1 回で書く)。ワークスペースのログでは、削除の f の下の行かだけで重なりを決める。
func (in incidentals) of(ref change.Ref) func(change.Record) bool {
	return func(c change.Record) bool {
		if ref.Kind != change.KindWorkspace && in.deleteOps[c.Op] {
			return true
		}
		return ref.Kind == change.KindItem && c.T == change.Set && c.F == placeField && in.ancestorPlaced(ref.ID, c.Op)
	}
}

// ancestorPlaced は、アイテム id の祖先(いまの親をたどる)のどれかに、op が set place を書いたか。
// 親が循環していても止まる。
func (in incidentals) ancestorPlaced(id item.ID, op change.ID) bool {
	seen := map[item.ID]bool{id: true}
	it := in.items[id]
	for it.Place.Parent != nil {
		p := *it.Place.Parent
		if seen[p] {
			return false
		}
		seen[p] = true
		if in.placed[op][p] {
			return true
		}
		next, ok := in.items[p]
		if !ok {
			return false
		}
		it = next
	}
	return false
}
