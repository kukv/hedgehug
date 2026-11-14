package bulkchange

import (
	"context"
	"errors"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// frameKey は、スプリントへ移すアイテムの、ワークスペースと移動先(空はバックログ)。
type frameKey struct {
	ws     workspace.ID
	sprint change.ID
}

// writeDrafts は、変えるアイテムの行を書く。スプリントへ移すアイテムには、同じ行の束に順位の行を足す。
func writeDrafts(tx port.Tx, js []judged) error {
	ranks, err := sprintRanks(tx, js)
	if err != nil {
		return err
	}
	for _, j := range js {
		if !j.changes() {
			continue
		}
		ds := slices.Clone(j.v.drafts)
		if d, ok := ranks[j.t.ID]; ok {
			ds = append(ds, d)
		}
		if err := tx.WriteItems().Append(j.t.ID, ds...); err != nil {
			return err
		}
	}
	return nil
}

// sprintRanks は、スプリントへ移すアイテムの順位の行(F2b2-8)。ワークスペースと移動先ごとに、移動先の枠の
// 末尾(書く前の値)から、いまの並び順のまま続ける。移動先の枠が空なら順位を変えない(app/planning の SetSprint と同じ)。
func sprintRanks(r port.Reader, js []judged) (map[item.ID]change.Draft, error) {
	var keys []frameKey
	moved := map[frameKey][]judged{}
	for _, j := range js {
		if !j.changes() || j.v.sprint == nil {
			continue
		}
		k := frameKey{j.c.it.Place.Workspace, *j.v.sprint}
		if moved[k] == nil {
			keys = append(keys, k)
		}
		moved[k] = append(moved[k], j)
	}
	out := map[item.ID]change.Draft{}
	for _, k := range keys {
		its, err := workspaceItems(r, k.ws)
		if err != nil {
			return nil, err
		}
		w := moved[k][0].ws
		end := planning.FrameEnd(its, k.ws, k.sprint, w.plan, w.settings)
		if end == "" {
			continue
		}
		cur := map[item.ID]item.Item{}
		var works []item.Item
		for _, j := range moved[k] {
			cur[j.t.ID] = j.c.it
			works = append(works, j.c.it)
		}
		for id, rk := range planning.AppendWorks(works, end) {
			out[id] = change.NewSet("rank", cur[id].Rank, rk)
		}
	}
	return out, nil
}

// workspaceItems は、ワークスペース ws のすべてのアイテム(app/planning の items と同じ)。
func workspaceItems(r port.Reader, ws workspace.ID) ([]item.Item, error) {
	ps, err := r.Projects().List(ws)
	if err != nil {
		return nil, err
	}
	var out []item.Item
	for _, p := range ps {
		its, err := r.Items().List(p.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, its...)
	}
	return out, nil
}

// offBacklog は、保存のあとに、変えたアイテムを読み直し、作業項目で、完了していない使えるスプリントに
// 入っておらず、分類が完了・中止のものを数えて n に足す(F2b2-17。自動化の変更を含む、保存のあとの値)。
func (s Service) offBacklog(ctx context.Context, store port.StoreID, js []judged, n *OffBacklog) error {
	return s.Stores.Read(ctx, store, func(r port.Reader) error {
		for _, j := range js {
			if !j.changes() {
				continue
			}
			it, err := port.LiveItem(r, j.t.ID)
			if errors.Is(err, port.ErrNotFound) {
				continue
			}
			if err != nil {
				return err
			}
			n.add(j.ws, it)
		}
		return nil
	})
}

// add は、保存のあとのアイテム it が、作業項目で、完了していない使えるスプリントに入っておらず、分類が完了・中止なら数える。
func (n *OffBacklog) add(w *wsChoices, it item.Item) {
	if sp, ok := w.plan.UsableSprint(it.Sprint); !planning.IsWork(it.Kind) || ok && sp.State != planning.Closed {
		return
	}
	switch planning.Category(w.settings, it.Status) {
	case workspace.CategoryDone:
		n.Done++
	case workspace.CategoryCanceled:
		n.Canceled++
	}
}
