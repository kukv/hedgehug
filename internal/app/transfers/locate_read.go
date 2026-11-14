package transfers

import (
	"context"
	"slices"
	"strconv"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// snapshot は、旧 ID の解決のために読んだ、表示中の 1 つの Store の中身。
type snapshot struct {
	info       port.StoreInfo
	workspaces []wsEntry // サイドバーの並び(rank の昇順、同じなら内部 ID の順)
	wsEntries  []entry   // ワークスペース(削除済みを含む)を番号 0 の entry にしたもの(B5c-13)
	items      []entry
	projects   []entry
	records    []transfer.Record
	purged     []port.Purged
}

type wsEntry struct {
	ws  workspace.Workspace
	old []string // 旧い接頭辞
}

// entry は、アイテムかプロジェクト(削除済みを含む)。project は、アイテムならそのプロジェクト、プロジェクトなら自身。
// live は、それ自身もプロジェクトもワークスペースも削除されていないこと(画面のアドレスの解決 items.FindByNumber・
// port.LiveProject と同じく、削除済みのプロジェクト・ワークスペース(B5b)のものは開かない)。
type entry struct {
	id      change.ID
	ws      workspace.ID
	project project.ID
	number  int
	live    bool
}

// kind は、旧 ID で探すものの種類。entity は転送記録の Entity、sep は画面の ID の接頭辞と番号のあいだ(HH-12・HH-P1)。
// ワークスペースは番号を持たず(0 として扱う)、画面の ID は接頭辞そのもの。
type kind struct{ entity, sep string }

var (
	itemKind      = kind{entity: transfer.EntityItem, sep: "-"}
	projectKind   = kind{entity: transfer.EntityProject, sep: "-P"}
	workspaceKind = kind{entity: transfer.EntityWorkspace}
)

// display は、接頭辞 prefix・番号 n の画面の ID。ワークスペースは接頭辞。
func (k kind) display(prefix string, n int) string {
	if k == workspaceKind {
		return prefix
	}
	return prefix + k.sep + strconv.Itoa(n)
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
		snap.wsEntries = append(snap.wsEntries, entry{id: w.ID, ws: w.ID, live: w.Deleted == nil})
		if err := snap.appendEntries(r, w); err != nil {
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

// appendEntries は、ワークスペース ws のすべてのプロジェクトと、そのアイテム(どちらも削除済みを含む)を足す。
func (snap *snapshot) appendEntries(r port.Reader, ws workspace.Workspace) error {
	ps, err := r.Projects().List(ws.ID)
	if err != nil {
		return err
	}
	for _, p := range ps {
		live := p.Deleted == nil && ws.Deleted == nil
		snap.projects = append(snap.projects, entry{id: p.ID, ws: p.Place.Workspace, project: p.ID, number: p.Place.Number, live: live})
		its, err := r.Items().List(p.ID)
		if err != nil {
			return err
		}
		for _, it := range its {
			snap.items = append(snap.items, entry{id: it.ID, ws: it.Place.Workspace, project: it.Place.Project, number: it.Place.Number, live: live && it.Deleted == nil})
		}
	}
	return nil
}

// entries は、転送記録の Entity が entity のもの(アイテム・プロジェクト・ワークスペース)。ワークスペースは、削除済みを
// 含めて、番号 0 のものとして返す。
func (snap snapshot) entries(entity string) []entry {
	switch entity {
	case transfer.EntityProject:
		return snap.projects
	case transfer.EntityWorkspace:
		return snap.wsEntries
	}
	return snap.items
}

// numbered は、接頭辞が合う(match)ワークスペースごとの、種類 k・番号 n の削除されていないものを、ワークスペースの並びで返す。
func (snap snapshot) numbered(k kind, n int, match func(wsEntry) bool) []hit {
	var out []hit
	for _, w := range snap.workspaces {
		if !match(w) {
			continue
		}
		for _, e := range snap.entries(k.entity) {
			if e.live && e.ws == w.ws.ID && e.number == n {
				out = append(out, snap.hit(k, e))
				break
			}
		}
	}
	return out
}

// live は、種類 k・内部 ID id の削除されていないもの。
func (snap snapshot) live(k kind, id change.ID) (hit, bool) {
	for _, e := range snap.entries(k.entity) {
		if e.live && e.id == id {
			return snap.hit(k, e), true
		}
	}
	return hit{}, false
}

// hasDeleted は、転送記録の Entity が entity・内部 ID id のものが、削除済み(それ自身・プロジェクト・ワークスペースの
// どれかが削除済み)であるか。
func (snap snapshot) hasDeleted(entity string, id change.ID) bool {
	return slices.ContainsFunc(snap.entries(entity), func(e entry) bool { return !e.live && e.id == id })
}

func (snap snapshot) hasPurged(id change.ID) bool {
	return slices.ContainsFunc(snap.purged, func(p port.Purged) bool { return p.Target == id })
}

// hit は、旧 ID から見つかったもの(アイテム・プロジェクト・ワークスペース)と、そのいまの ID(ワークスペースはいまの Store 名。
// MovedFrom の Current になる。B5c-13)。
type hit struct {
	store   port.StoreID
	id      change.ID
	project project.ID
	current string
}

func (snap snapshot) hit(k kind, e entry) hit {
	prefix := ""
	for _, w := range snap.workspaces {
		if w.ws.ID == e.ws {
			prefix = w.ws.Prefix
		}
	}
	current := k.display(prefix, e.number)
	if k == workspaceKind {
		current = snap.info.Name
	}
	return hit{store: snap.info.ID, id: e.id, project: e.project, current: current}
}

// matching は、scope の転送記録のうち、種類 k(Entity)の旧 ID が display の行を、at の新しい順で返す。Store の中に、display の
// 接頭辞を旧い接頭辞に持つワークスペースがあれば、その Store の記録は、そのワークスペースのいまの接頭辞・ほかの旧い
// 接頭辞で書いた同じ番号の ID(HH-2 なら WEB-2)でも一致させる(接頭辞を変えたあとに移したもの)。ワークスペースの記録は、
// 記録の内部 ID のワークスペースを表示中のすべての Store all から探し、そのいまの接頭辞・旧い接頭辞が prefix でも一致させる
// (接頭辞を変えてから別の Store へ移したもの。B5c-13)。探すのは表示中の Store だけなので、移動先の Store が非表示なら、
// 接頭辞を変える前の接頭辞は記録の old と一致せず、その記録には当たらない(ほかの Store の同じ接頭辞のものを開く)。
// 非表示の Store の中身はこのマシンの表示に使わない(01「画面のアドレス」は表示中の Store を探す)ため。
func matching(k kind, scope, all []snapshot, prefix string, n int, display string) []transfer.Record {
	var out []transfer.Record
	for _, snap := range scope {
		ids := snap.aliases(k, prefix, n, display)
		for _, r := range snap.records {
			if r.Entity != k.entity {
				continue
			}
			if slices.Contains(ids, r.Old) || k == workspaceKind && slices.Contains(prefixesOf(all, r.Target), prefix) {
				out = append(out, r)
			}
		}
	}
	slices.SortStableFunc(out, func(a, b transfer.Record) int { return b.At.Compare(a.At) })
	return out
}

// aliases は、display と、接頭辞 prefix を旧い接頭辞に持つワークスペースの、いまの接頭辞・旧い接頭辞で書いた種類 k・番号 n の ID。
func (snap snapshot) aliases(k kind, prefix string, n int, display string) []string {
	ids := []string{display}
	for _, w := range snap.workspaces {
		if !slices.Contains(w.old, prefix) {
			continue
		}
		for _, p := range append([]string{w.ws.Prefix}, w.old...) {
			ids = append(ids, k.display(p, n))
		}
	}
	return ids
}
