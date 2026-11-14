// Package search は、表示中のすべての Store のアイテムをクエリで検索する(04「検索」)。クエリの解析・照合・並びは
// domain/query が決め、ここは Store を読んで照合の材料(query.Facts)を作る。
package search

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/query"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

// Service は、検索が使う口。SyncTargets は、非表示を含むこのマシンの Store(追加プロパティのキーを探す)。
type Service struct {
	Stores      port.Stores
	StoreList   port.StoreList
	SyncTargets port.SyncTargets
	Identity    port.Identity
	Inbox       port.InboxBook
	Clock       port.Clock
}

// entry は、照合の対象のアイテム 1 件。daysLeft は削除済みのときだけで、完全に削除されるまでの残り日数。
type entry struct {
	info     port.StoreInfo
	facts    query.Facts
	daysLeft *int
}

// entries は、表示中の Store を StoreList の順に 1 つずつ読み(読み取りを入れ子にしない)、アイテムごとの材料を返す。
// q.Deleted なら削除済みのアイテム(ワークスペースが削除されていないもの)、でなければ削除されていないアイテム。
// 旧 ID は、すべての Store の転送記録を集めてから探す(別の Store へ移しても内部 ID は変わらない)。
func (s Service) entries(ctx context.Context, q query.Query) ([]entry, error) {
	infos, err := s.StoreList.List(ctx)
	if err != nil {
		return nil, err
	}
	now := s.Clock.Now()
	snaps := make([]*snapshot, 0, len(infos))
	var records []transfer.Record
	for i, info := range infos {
		snap, err := s.read(ctx, info, i, q)
		if err != nil {
			return nil, err
		}
		records = append(records, snap.records...)
		snaps = append(snaps, snap)
	}
	today := planning.Today(now)
	var out []entry
	for _, snap := range snaps {
		for _, it := range snap.targets {
			e := entry{info: snap.info, facts: snap.facts(it, records, q, now)}
			if q.Deleted {
				n := it.Deleted.DaysLeft(snap.retention, today)
				e.daysLeft = &n
			}
			out = append(out, e)
		}
	}
	return out, nil
}

// read は、Store info を読み、Store の外の口(あなた・受信箱)の分を足す。
func (s Service) read(ctx context.Context, info port.StoreInfo, order int, q query.Query) (*snapshot, error) {
	var snap *snapshot
	if err := s.Stores.Read(ctx, info.ID, func(r port.Reader) (err error) {
		snap, err = readSnapshot(r, q)
		return err
	}); err != nil {
		return nil, err
	}
	snap.info, snap.order = info, order
	snap.me = s.me(ctx, info.ID)
	if q.NeedsInbox {
		var err error
		if snap.overwritten, err = s.overwritten(ctx, info.ID); err != nil {
			return nil, err
		}
	}
	return snap, nil
}

// me は、Store の「あなた」。読めないか空なら空(@me は何にも一致しない。E1-5)。
func (s Service) me(ctx context.Context, store port.StoreID) string {
	login, err := s.Identity.Me(ctx, store)
	if err != nil {
		return ""
	}
	return login
}
