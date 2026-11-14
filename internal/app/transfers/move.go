package transfers

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// MoveInput は、移す先。ToParent は移動先のプロジェクトの Epic(Epic を移すときは nil)。StatusChoices は、
// 移動先で使えない移動元のステータス → 選んだ置き換え先。
type MoveInput struct {
	ToStore       port.StoreID
	ToWorkspace   workspace.ID
	ToProject     project.ID
	ToParent      *item.ID
	StatusChoices map[change.ID]change.ID
}

// Moved は、移した結果。Item は移したアイテムの内部 ID(変わらない)、Display は新しい ID、Emptied は合う値がなく
// 空になった項目の名前(移動元のプロパティの定義の順)。
type Moved struct {
	ToStore port.StoreID
	Item    item.ID
	Display string
	Emptied []string
}

// Move は、アイテム id を、下のもの(削除済みを含む)ごと別のワークスペースへ移す(02「別のワークスペースと別の
// Store への移動」、A5a-3)。同じ Store の中は 1 回の保存で、番号を振り直し、値を引き直し、転送記録を残す。
// 移すアイテムは移動先の親(Epic ならプロジェクト)の削除されていない子の末尾に入り、移す作業項目は移動元の並び順の
// まま移動先の作業項目の末尾に続く(F2a-6)。下のものの親子と Step の順位は変えない。自動化は動かさない(03「自動化」)。
// 移動元の Store が閲覧のみなら「閲覧のみの Store です」で断る。
// 移動先の Store が表示中でないなら port.ErrStoreNotFound、閲覧のみ・選べない移すもの・移動先なら port.ErrNotFound。
// 別の Store への移動は moveAcross。
func (s Service) Move(ctx context.Context, store port.StoreID, id item.ID, in MoveInput) (Moved, error) {
	info, err := s.storeInfo(ctx, store)
	if err != nil {
		return Moved{}, err
	}
	if info.ReadOnly {
		return Moved{}, refusal.New("閲覧のみの Store です")
	}
	if err := s.checkDestination(ctx, in.ToStore); err != nil {
		return Moved{}, err
	}
	if in.ToStore != store {
		return s.moveAcross(ctx, store, id, in)
	}
	var summary string
	err = s.Stores.Read(ctx, store, func(r port.Reader) error {
		p, err := plan(r, id, in, readMovable)
		if err != nil {
			return err
		}
		summary = p.summary()
		return nil
	})
	if err != nil {
		return Moved{}, err
	}
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return Moved{}, err
	}
	var moved Moved
	op := port.Operation{Actor: change.User(me), Summary: summary}
	err = s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		// 別の画面で削除されていたら、同じワークスペースの中の移動(items.Move)と同じく、削除を取り消してから移す(06)。
		if err := port.ReviveItem(ctx, tx, id); err != nil {
			return err
		}
		p, err := plan(tx, id, in, readSource)
		if err != nil {
			return err
		}
		moved = Moved{ToStore: store, Item: id, Display: p.display(p.src.set.Root), Emptied: emptiedOf(p.remaps, p.src.settings)}
		return writeWithin(tx, p, store)
	})
	if err != nil {
		return Moved{}, err
	}
	return moved, nil
}

// movePlan は、移動の材料。保存の前(アクティビティの 1 行)と保存の中の両方で plan が作る。
type movePlan struct {
	src     source
	dst     destination
	parent  *item.ID
	ranks   map[item.ID]string
	numbers map[item.ID]int
	remaps  map[item.ID]transfer.RemapResult
}

// plan は、移すもの(read で読む)と移動先を検査して読む。移動先が移動元と同じワークスペースなら port.ErrNotFound
// (ほかの検査は target)。
func plan(r port.Reader, id item.ID, in MoveInput, read func(port.Reader, item.ID) (source, error)) (movePlan, error) {
	src, err := read(r, id)
	if err != nil {
		return movePlan{}, err
	}
	dst, ranks, err := target(r, src.set.Root, src.items(), in)
	if err != nil {
		return movePlan{}, err
	}
	if dst.ws.ID == src.ws.ID {
		return movePlan{}, port.ErrNotFound
	}
	p := movePlan{
		src: src, dst: dst, parent: in.ToParent, ranks: ranks,
		numbers: transfer.Number(src.set.Root, src.set.Below, dst.next), remaps: map[item.ID]transfer.RemapResult{},
	}
	for _, it := range src.items() {
		p.remaps[it.ID] = transfer.Remap(it, src.settings, dst.settings, dst.rules, nil, in.StatusChoices)
	}
	return p, nil
}

// target は、移動先を r から読んで検査し、移すもの moved(移すアイテム root を含む)の移動先での順位(workRanks)を返す
// (同じ Store と別の Store で共有)。Epic に ToParent がある、Epic 以外で ToParent が移動先のプロジェクトの削除されていない
// Epic でないなら port.ErrNotFound。
func target(r port.Reader, root item.Item, moved []item.Item, in MoveInput) (destination, map[item.ID]string, error) {
	dst, err := readDestination(r, in)
	if err != nil {
		return destination{}, nil, err
	}
	if err := checkParent(r, root, in); err != nil {
		return destination{}, nil, err
	}
	siblings, err := port.LiveItems(r, dst.project.ID)
	if err != nil {
		return destination{}, nil, err
	}
	wsItems, err := workspaceItems(r, dst.ws.ID)
	if err != nil {
		return destination{}, nil, err
	}
	return dst, workRanks(dst.ws.ID, wsItems, moved, &root, lastRank(siblings, in.ToParent)), nil
}

// checkParent は、移す先の親を確かめる。
func checkParent(r port.Reader, root item.Item, in MoveInput) error {
	if root.Kind == item.Epic {
		if in.ToParent != nil {
			return port.ErrNotFound
		}
		return nil
	}
	if in.ToParent == nil {
		return port.ErrNotFound
	}
	e, err := port.LiveItem(r, *in.ToParent)
	if err != nil {
		return err
	}
	if e.Kind != item.Epic || e.Place.Project != in.ToProject {
		return port.ErrNotFound
	}
	return nil
}

// summary は、アクティビティの 1 行「{旧 ID} を {移動元} から {移動先} へ移動」(A5a-11)。
func (p movePlan) summary() string {
	root := p.src.set.Root
	return fmt.Sprintf("%s を %s から %s へ移動", root.DisplayID(p.src.ws.Prefix), p.src.ws.Name, p.dst.ws.Name)
}

// display は、移したあとの it の ID。
func (p movePlan) display(it item.Item) string { return displayID(p.dst.ws.Prefix, p.numbers[it.ID]) }

// emptiedOf は、すべての引き直しで空になった項目の名前を、移動元のプロパティの定義 from の順で返す。
func emptiedOf(remaps map[item.ID]transfer.RemapResult, from workspace.Settings) []string {
	got := map[string]bool{}
	for _, r := range remaps {
		for _, name := range r.Emptied {
			got[name] = true
		}
	}
	var out []string
	for _, prop := range from.Properties {
		if got[prop.Name] {
			out = append(out, prop.Name)
			delete(got, prop.Name)
		}
	}
	return out
}

// writeWithin は、同じ Store の中の移動の行を書く(A5a-3・F2a-6): アイテムごとに place(移すアイテムと作業項目は
// rank も)と引き直しの行、移動先のワークスペースに番号ごとの number の行、アイテムごとに転送記録。
func writeWithin(tx port.Tx, p movePlan, store port.StoreID) error {
	var errs []error
	for _, it := range p.src.items() {
		drafts := placeLines(it, it.ID == p.src.set.Root.ID, p.dst, p.parent, p.numbers[it.ID], p.ranks)
		drafts = append(drafts, p.remaps[it.ID].Drafts...)
		errs = append(errs, tx.WriteItems().Append(it.ID, drafts...))
	}
	errs = append(errs, tx.WriteWorkspaces().Append(p.dst.ws.ID, numbering.Drafts(numbering.Items, slices.Collect(maps.Values(p.numbers)))...))
	for _, it := range p.src.items() {
		errs = append(errs, tx.WriteTransfers().Append(transfer.Record{
			Entity: transfer.EntityItem, Target: it.ID, Old: it.DisplayID(p.src.ws.Prefix), New: p.display(it), ToStore: string(store),
		}))
	}
	return errors.Join(errs...)
}

// placeLines は、it を移動先 dst の番号 n へ置く行。移すアイテム(root)だけ親を parent にし、ranks(workRanks)の順位が
// 今と違えば rank も書く。
func placeLines(it item.Item, root bool, dst destination, parent *item.ID, n int, ranks map[item.ID]string) []change.Draft {
	place := item.Place{Workspace: dst.ws.ID, Project: dst.project.ID, Parent: it.Place.Parent, Number: n}
	if root {
		place.Parent = parent
	}
	return append([]change.Draft{change.NewSet("place", it.Place, place)}, rankLine(it, ranks)...)
}

// workRanks は、別のワークスペース ws へ移す moved の、移動先での順位(F2a-6)。dstItems は移動先のワークスペースのアイテム。
// moved の作業項目は、移動元の並び順のまま、移動先の作業項目の末尾に続く。root は移すアイテム(プロジェクトの移動は空)で、
// siblingEnd は移動先の親の削除されていない子の末尾。root が作業項目なら、その順位は親の子の末尾も満たす(F2a-5 の値から
// 続ける)。作業項目でない root(Epic)は親の子の末尾。ほかのもの(Step)は入らない。
// moved の削除済みの作業項目にも順位を付ける。別の Store への移動は、別の画面で削除された root の順位を削除済みの写しで
// 決め、移動先で削除を取り消すため。削除済みのものは表示されず、復元すると移動元の並び順のまま戻る。
func workRanks(ws workspace.ID, dstItems, moved []item.Item, root *item.Item, siblingEnd string) map[item.ID]string {
	start := planning.WorkEnd(dstItems, ws)
	if root != nil && planning.IsWork(root.Kind) {
		start = max(start, siblingEnd)
	}
	ranks := planning.AppendWorks(moved, start)
	if root != nil && !planning.IsWork(root.Kind) {
		ranks[root.ID] = rank.After(siblingEnd)
	}
	return ranks
}

// workspaceItems は、ワークスペース ws の全プロジェクトのアイテム(削除済みも含む)。
func workspaceItems(r port.Reader, ws workspace.ID) ([]item.Item, error) {
	ps, err := r.Projects().List(ws)
	if err != nil {
		return nil, err
	}
	var all []item.Item
	for _, p := range ps {
		its, err := r.Items().List(p.ID)
		if err != nil {
			return nil, err
		}
		all = append(all, its...)
	}
	return all, nil
}

// rankLine は、順位を ranks の値へ変える行。ranks にないか、今と同じなら書かない。
func rankLine(it item.Item, ranks map[item.ID]string) []change.Draft {
	r, ok := ranks[it.ID]
	if !ok || r == it.Rank {
		return nil
	}
	return []change.Draft{change.NewSet("rank", it.Rank, r)}
}
