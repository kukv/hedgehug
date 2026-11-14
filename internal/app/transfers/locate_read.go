package transfers

import (
	"context"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// snapshot は、旧 ID の解決のために読んだ、表示中の 1 つの Store の中身。
type snapshot struct {
	info       port.StoreInfo
	workspaces []wsEntry // サイドバーの並び(rank の昇順、同じなら内部 ID の順)
	items      []itemEntry
	records    []transfer.Record
	purged     []port.Purged
}

type wsEntry struct {
	ws  workspace.Workspace
	old []string // 旧い接頭辞
}

// itemEntry は、アイテム(削除済みを含む)。live は、アイテムもプロジェクトもワークスペースも削除されていないこと
// (画面のアドレスの解決 items.FindByNumber と同じく、削除済みのプロジェクト・ワークスペース(B5b)のアイテムは開かない)。
type itemEntry struct {
	it   item.Item
	live bool
}

// snapshots は、表示中のすべての Store を、サイドバーの並びで 1 つずつ読む(読み取りを入れ子にしない)。
func (s Service) snapshots(ctx context.Context) ([]snapshot, error) {
	infos, err := s.StoreList.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]snapshot, 0, len(infos))
	for _, info := range infos {
		var snap snapshot
		if err := s.Stores.Read(ctx, info.ID, func(r port.Reader) (err error) {
			snap, err = readSnapshot(r, info)
			return err
		}); err != nil {
			return nil, err
		}
		out = append(out, snap)
	}
	return out, nil
}

func readSnapshot(r port.Reader, info port.StoreInfo) (snapshot, error) {
	snap := snapshot{info: info}
	wss, err := r.Workspaces().List()
	if err != nil {
		return snap, err
	}
	ranks := map[workspace.ID]string{}
	for _, w := range wss {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: w.ID})
		if err != nil {
			return snap, err
		}
		ranks[w.ID] = workspace.RankOf(log)
		snap.workspaces = append(snap.workspaces, wsEntry{ws: w, old: workspace.OldPrefixes(log)})
		if snap.items, err = appendItems(r, w, snap.items); err != nil {
			return snap, err
		}
	}
	sortByWorkspaceRank(snap.workspaces, ranks, func(e wsEntry) workspace.ID { return e.ws.ID })
	if snap.records, err = r.Transfers().List(); err != nil {
		return snap, err
	}
	snap.purged, err = r.Purged().List()
	return snap, err
}

// appendItems は、ワークスペース ws のすべてのプロジェクト(削除済みを含む)のアイテムを足す。
func appendItems(r port.Reader, ws workspace.Workspace, out []itemEntry) ([]itemEntry, error) {
	ps, err := r.Projects().List(ws.ID)
	if err != nil {
		return out, err
	}
	for _, p := range ps {
		its, err := r.Items().List(p.ID)
		if err != nil {
			return out, err
		}
		for _, it := range its {
			out = append(out, itemEntry{it: it, live: it.Deleted == nil && p.Deleted == nil && ws.Deleted == nil})
		}
	}
	return out, nil
}

// numbered は、接頭辞が合う(match)ワークスペースごとの、番号 n の削除されていないアイテムを、ワークスペースの並びで返す。
func (snap snapshot) numbered(n int, match func(wsEntry) bool) []hit {
	var out []hit
	for _, w := range snap.workspaces {
		if !match(w) {
			continue
		}
		for _, e := range snap.items {
			if e.live && e.it.Place.Workspace == w.ws.ID && e.it.Place.Number == n {
				out = append(out, snap.hit(e.it))
				break
			}
		}
	}
	return out
}

// live は、内部 ID id の削除されていないアイテム。
func (snap snapshot) live(id item.ID) (hit, bool) {
	for _, e := range snap.items {
		if e.live && e.it.ID == id {
			return snap.hit(e.it), true
		}
	}
	return hit{}, false
}

// hasDeleted は、内部 ID id のアイテムが、削除済み(アイテム・プロジェクト・ワークスペースのどれかが削除済み)であるか。
func (snap snapshot) hasDeleted(id item.ID) bool {
	return slices.ContainsFunc(snap.items, func(e itemEntry) bool { return !e.live && e.it.ID == id })
}

func (snap snapshot) hasPurged(id change.ID) bool {
	return slices.ContainsFunc(snap.purged, func(p port.Purged) bool { return p.Target == id })
}

// hit は、旧 ID から見つかったアイテムと、そのいまの ID。
type hit struct {
	store   port.StoreID
	it      item.Item
	current string
}

func (snap snapshot) hit(it item.Item) hit {
	prefix := ""
	for _, w := range snap.workspaces {
		if w.ws.ID == it.Place.Workspace {
			prefix = w.ws.Prefix
		}
	}
	return hit{store: snap.info.ID, it: it, current: it.DisplayID(prefix)}
}

func (h hit) located(m *MovedFrom) Located {
	return Located{Found: true, Store: h.store, Project: h.it.Place.Project, Item: h.it.ID, MovedFrom: m}
}
