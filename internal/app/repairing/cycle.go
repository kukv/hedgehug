package repairing

import (
	"context"
	"errors"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/repair"
)

// move は、輪を切るために 1 件を動かす修復 1 つ。
type move struct {
	it      item.Item
	to      item.Place
	rank    string
	epic    *item.Item // 一緒に作る「修復で作った Epic」。作らなければ nil
	display string     // 動かすものの画面の ID
	dest    string     // 行き先の画面の ID(Epic、またはプロジェクトの直下ならプロジェクト)
}

// breakCycles は、削除されていないアイテムの親の輪を 1 つずつ切る(決定 C3a-14・C3a-15)。輪の中で動かすものを
// 08「親がない」の行き先へ動かし、続けて、輪に入っていたほかのアイテムで親が種類の規則に合わないものも動かす。
// 1 件ごとに 1 回保存し、輪がなくなるまで読み直して繰り返す。動かすもののプロジェクトが読めない輪は直さず、
// その輪のアイテムを除いて次の輪を探す(輪は互いに重ならないので、ほかの輪は除かれない)。
func breakCycles(ctx context.Context, w port.RepairWriter) error {
	skipped := map[item.ID]bool{}
	var prev []item.ID
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		all, err := readAllItems(w)
		if err != nil {
			return err
		}
		all = slices.DeleteFunc(all, func(it item.Item) bool { return skipped[it.ID] })
		ring, ok := repair.Cycle(all)
		if !ok {
			return nil
		}
		// 直さずに飛ばした輪は all から除くので、同じ輪がまた来るのは、書いても切れなかったときだけ。
		if slices.Equal(ring, prev) {
			return errors.New("親の循環の修復が進みません")
		}
		prev = ring
		if err := breakRing(ctx, w, ring, all, skipped); err != nil {
			return err
		}
	}
}

// breakRing は、輪 ring を 1 つ切る。動かすもののプロジェクトが読めなければ、何も書かずに ring を skipped に入れる。
func breakRing(ctx context.Context, w port.RepairWriter, ring []item.ID, all []item.Item, skipped map[item.ID]bool) error {
	moved, err := moveOut(w, repair.Mover(ring, all))
	if err != nil {
		return err
	}
	if !moved {
		for _, id := range ring {
			skipped[id] = true
		}
		return nil
	}
	return fixMembers(ctx, w, ring)
}

// fixMembers は、切った輪 ring のほかのアイテムで、親が種類の規則に合わないものを、内部 ID の順に 1 件ずつ動かす。
// 1 件は 1 度しか見ないので、必ず止まる。
func fixMembers(ctx context.Context, w port.RepairWriter, ring []item.ID) error {
	done := map[item.ID]bool{}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		all, err := readAllItems(w)
		if err != nil {
			return err
		}
		id, ok := misplaced(ring, all, done)
		if !ok {
			return nil
		}
		done[id] = true
		if _, err := moveOut(w, id); err != nil {
			return err
		}
	}
}

// misplaced は、ring のうち done でない、削除されていないアイテムで、親の種類が item.CanPlace に合わない最初のもの。
// 親が見つからないか削除済みのものは、この修復では見ない。
func misplaced(ring []item.ID, all []item.Item, done map[item.ID]bool) (item.ID, bool) {
	byID := make(map[item.ID]item.Item, len(all))
	for _, it := range all {
		byID[it.ID] = it
	}
	for _, id := range ring {
		it, ok := byID[id]
		if done[id] || !ok || it.Deleted != nil {
			continue
		}
		var parent *item.Kind
		if it.Place.Parent != nil {
			p, ok := byID[*it.Place.Parent]
			if !ok || p.Deleted != nil {
				continue
			}
			parent = &p.Kind
		}
		if !item.CanPlace(it.Kind, parent) {
			return id, true
		}
	}
	return "", false
}

// moveOut は、id を行き先へ動かす修復を 1 回保存する。プロジェクトが読めなければ、何も書かずに false を返す。
func moveOut(w port.RepairWriter, id item.ID) (bool, error) {
	purged, err := w.Purged()
	if err != nil {
		return false, err
	}
	var m move
	var ok bool
	err = w.Read(func(r port.Reader) error {
		var err error
		m, ok, err = planMove(r, id, purged)
		return err
	})
	if err != nil || !ok {
		return false, err
	}
	op := port.Operation{Actor: syncActor, Summary: m.display + " を " + m.dest + " の下へ移動(親の循環)"}
	return true, w.Write(op, func(tx port.Tx) error {
		var errs []error
		if m.epic != nil {
			errs = append(errs,
				tx.WriteWorkspaces().Append(m.epic.Place.Workspace, numbering.Draft(numbering.Items, m.epic.Place.Number)),
				tx.WriteItems().Append(m.epic.ID, item.Create(*m.epic)),
			)
		}
		ds := []change.Draft{change.NewSet(placeField, m.it.Place, m.to), change.NewSet(rankField, m.it.Rank, m.rank)}
		if m.it.Kind == item.Step {
			ds = append(ds, change.NewSet("kind", item.Step, item.Task))
		}
		return errors.Join(append(errs, tx.WriteItems().Append(m.it.ID, ds...))...)
	})
}

// planMove は、id の行き先(08「親がない」)を決める。プロジェクトが読めなければ(port.ErrNotFound)false。
// purged は、完全に削除したものの内部 ID(作る Epic の内部 ID に使わない)。
func planMove(r port.Reader, id item.ID, purged map[change.ID]bool) (move, bool, error) {
	it, err := r.Items().Get(id)
	if err != nil {
		return move{}, false, err
	}
	p, err := r.Projects().Get(it.Place.Project)
	if errors.Is(err, port.ErrNotFound) {
		return move{}, false, nil
	}
	if err != nil {
		return move{}, false, err
	}
	its, err := r.Items().List(p.ID)
	if err != nil {
		return move{}, false, err
	}
	ws, err := r.Workspaces().Get(it.Place.Workspace)
	if err != nil {
		return move{}, false, err
	}
	m := move{it: it, to: it.Place, display: it.DisplayID(ws.Prefix)}
	d := repair.DestinationOf(it, its)
	switch {
	case d.NeedEpic:
		epic, err := newEpic(r, p, its, purged)
		if err != nil {
			return move{}, false, err
		}
		m.epic, m.to.Parent, m.dest = &epic, &epic.ID, epic.DisplayID(ws.Prefix)
	case d.Parent == nil:
		m.to.Parent, m.dest = nil, p.DisplayID(ws.Prefix)
	default:
		i := slices.IndexFunc(its, func(e item.Item) bool { return e.ID == *d.Parent })
		m.to.Parent, m.dest = d.Parent, its[i].DisplayID(ws.Prefix)
	}
	m.rank = rank.After(lastLiveRank(its, m.to.Parent, it.ID))
	return m, true, nil
}

// newEpic は、プロジェクト p に作る「修復で作った Epic」(決定 C3a-15)。番号はワークスペースの次の番号、
// 順位はプロジェクトの直下の削除されていないものの末尾。ステータスと優先度は、作成と同じく初期値にする。
// 内部 ID は、アイテムがある(削除済みを含む)か、完全に削除したもの(purged)なら使わない。
func newEpic(r port.Reader, p project.Project, its []item.Item, purged map[change.ID]bool) (item.Item, error) {
	wsLog, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: p.Place.Workspace})
	if err != nil {
		return item.Item{}, err
	}
	st, err := r.Workspaces().Settings(p.Place.Workspace)
	if err != nil {
		return item.Item{}, err
	}
	id := repair.EpicID(p.ID, func(id change.ID) bool {
		if purged[id] {
			return true
		}
		_, err := r.Items().Get(id)
		return !errors.Is(err, port.ErrNotFound)
	})
	if id == "" {
		return item.Item{}, errors.New("修復で作る Epic の内部 ID がすべて使われています")
	}
	return item.Item{
		ID:       id,
		Place:    item.Place{Workspace: p.Place.Workspace, Project: p.ID, Number: numbering.Next(wsLog, numbering.Items)},
		Kind:     item.Epic,
		Rank:     rank.After(lastLiveRank(its, nil, "")),
		Title:    repair.RepairEpicTitle,
		Status:   st.InitialStatus,
		Priority: st.InitialPriority,
	}, nil
}

// lastLiveRank は、parent の子(nil はプロジェクトの直下)で削除されていないもののうち、self を除いて
// item.SortSiblings で並べた末尾の順位。なければ空。
func lastLiveRank(its []item.Item, parent *item.ID, self item.ID) string {
	var kids []item.Item
	for _, it := range item.Live(its) {
		same := (parent == nil && it.Place.Parent == nil) ||
			(parent != nil && it.Place.Parent != nil && *it.Place.Parent == *parent)
		if same && it.ID != self {
			kids = append(kids, it)
		}
	}
	if len(kids) == 0 {
		return ""
	}
	item.SortSiblings(kids)
	return kids[len(kids)-1].Rank
}

// readAllItems は、すべてのワークスペースのプロジェクトのアイテムを読む(削除済みを含む)。
func readAllItems(w port.RepairWriter) ([]item.Item, error) {
	var all []item.Item
	err := w.Read(func(r port.Reader) error {
		wss, err := r.Workspaces().List()
		if err != nil {
			return err
		}
		for _, ws := range wss {
			ps, err := r.Projects().List(ws.ID)
			if err != nil {
				return err
			}
			its, err := itemsOf(r, ps)
			if err != nil {
				return err
			}
			all = append(all, its...)
		}
		return nil
	})
	return all, err
}
