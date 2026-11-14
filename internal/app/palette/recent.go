package palette

import (
	"context"
	"errors"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

const (
	recentKeep  = 50 // 覚えておく件数
	recentShown = 8  // 返す件数(02「最近開いたもの」の上限)
)

// RecentItem は、最近開いたアイテム 1 件。Store・Display は、いま見つかった Store と、そこでのいまの ID。
type RecentItem struct {
	Store        port.StoreID
	Item         item.ID
	Display      string
	Title        string
	ProjectTitle string
}

// Opened は、Store store のアイテム id を開いたと、このマシンに覚える(新しい 50 件。A5c-13)。
func (s Service) Opened(ctx context.Context, store port.StoreID, id change.ID) error {
	return s.RecentItems.Touch(ctx, store, id, s.Clock.Now(), recentKeep)
}

// Recent は、覚えたアイテムを新しい順に、表示中の Store の削除されていないものから探して 8 件まで返す(A5c-13)。
// 探す Store は、覚えた Store を先に、なければほかの表示中の Store(別の Store へ移しても内部 ID は同じ)。
// 見つからないもののうち、完全に削除したものと、このマシンのどの Store にもないものは記録から消す。削除済みのものと
// 非表示の Store のものは、消さずに飛ばす。
func (s Service) Recent(ctx context.Context) ([]RecentItem, error) {
	rows, err := s.RecentItems.List(ctx)
	if err != nil {
		return nil, err
	}
	snaps, err := s.snapshots(ctx)
	if err != nil {
		return nil, err
	}
	live := liveOf(snaps)
	out := []RecentItem{}
	var missing []change.ID
	for _, r := range rows {
		it, ok := live.find(r.Store, r.Item)
		switch {
		case !ok:
			missing = append(missing, r.Item)
		case len(out) < recentShown:
			out = append(out, it)
		}
	}
	gone, err := s.vanished(ctx, missing)
	if err != nil {
		return nil, err
	}
	if len(gone) > 0 {
		if err := s.RecentItems.Forget(ctx, gone); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// liveItems は、表示中の Store ごとの、削除されていないアイテム(order は StoreList の並び)。
type liveItems struct {
	order  []port.StoreID
	stores map[port.StoreID]map[item.ID]RecentItem
}

func liveOf(snaps []snapshot) liveItems {
	l := liveItems{stores: map[port.StoreID]map[item.ID]RecentItem{}}
	for _, snap := range snaps {
		id := snap.info.ID
		m := map[item.ID]RecentItem{}
		for _, w := range snap.workspaces {
			for _, p := range w.projects {
				for _, it := range p.items {
					m[it.ID] = RecentItem{Store: id, Item: it.ID, Display: it.DisplayID(w.ws.Prefix), Title: it.Title, ProjectTitle: p.project.Title}
				}
			}
		}
		l.order = append(l.order, id)
		l.stores[id] = m
	}
	return l
}

// find は、アイテム id を、Store store から、なければほかの表示中の Store から並びの順に探す。
func (l liveItems) find(store port.StoreID, id item.ID) (RecentItem, bool) {
	if it, ok := l.stores[store][id]; ok {
		return it, true
	}
	for _, other := range l.order {
		if it, ok := l.stores[other][id]; ok {
			return it, true
		}
	}
	return RecentItem{}, false
}

// vanished は、ids のうち、このマシンのどれかの Store(SyncTargets。非表示を含む)の完全に削除したものの記録にあるか、
// どの Store にもない(削除済みとしても残っていない)ものを、ids の並びで返す。Store は 1 つずつ読む。
func (s Service) vanished(ctx context.Context, ids []change.ID) ([]change.ID, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	targets, err := s.SyncTargets.List(ctx)
	if err != nil {
		return nil, err
	}
	purged, exists := map[change.ID]bool{}, map[change.ID]bool{}
	for _, t := range targets {
		if err := s.Stores.Read(ctx, t.ID, func(r port.Reader) error {
			return scanStore(r, ids, purged, exists)
		}); err != nil {
			return nil, err
		}
	}
	var out []change.ID
	for _, id := range ids {
		if purged[id] || !exists[id] {
			out = append(out, id)
		}
	}
	return out, nil
}

// scanStore は、1 つの Store の、完全に削除したものの記録を purged に、ids のうちあるもの(削除済みを含む)を exists に足す。
func scanStore(r port.Reader, ids []change.ID, purged, exists map[change.ID]bool) error {
	ps, err := r.Purged().List()
	if err != nil {
		return err
	}
	for _, p := range ps {
		purged[p.Target] = true
	}
	for _, id := range ids {
		_, err := r.Items().Get(id)
		switch {
		case err == nil:
			exists[id] = true
		case !errors.Is(err, port.ErrNotFound):
			return err
		}
	}
	return nil
}
