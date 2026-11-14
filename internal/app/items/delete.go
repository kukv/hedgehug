package items

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// DeletePreview は、削除の確認ダイアログの中身(02「削除」。A4b-7)。Count は、下にある削除されていないものの数。
// Kinds は、その種類を Epic・Story・Task・Bug・Step の順に、重ねずに並べたもの。
type DeletePreview struct {
	Count         int
	Kinds         []item.Kind
	RetentionDays int
}

// kindOrder は、{種類名の一覧} の並び。
var kindOrder = []item.Kind{item.Epic, item.Story, item.Task, item.Bug, item.Step}

// liveBelow は、id のアイテムと、その下の削除されていないものすべて(子・孫。深さ優先)を返す。
// id が削除済みなら port.ErrNotFound。削除済みの子の下にいる削除されていないものも含む(A4b-1)。
func liveBelow(r port.Reader, id item.ID) (item.Item, []item.Item, error) {
	it, err := port.LiveItem(r, id)
	if err != nil {
		return item.Item{}, nil, err
	}
	all, err := r.Items().List(it.Place.Project)
	if err != nil {
		return item.Item{}, nil, err
	}
	return it, item.Live(item.Descendants(id, all)), nil
}

// DeletePreview は、id を削除したときに一緒に消えるものと、保持日数を返す。
// アイテムがない・削除済みなら port.ErrNotFound。
func (s Service) DeletePreview(ctx context.Context, store port.StoreID, id item.ID) (DeletePreview, error) {
	var out DeletePreview
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		_, below, err := liveBelow(r, id)
		if err != nil {
			return err
		}
		if out.RetentionDays, err = r.RetentionDays(); err != nil {
			return err
		}
		out.Count = len(below)
		for _, k := range kindOrder {
			for _, b := range below {
				if b.Kind == k {
					out.Kinds = append(out.Kinds, k)
					break
				}
			}
		}
		return nil
	})
	return out, err
}

// Delete は、アイテムと、その下の削除されていないものすべてを、同じ 1 回の保存で削除する(02「削除」)。
// 書くのは、それぞれのログの `delete`・`deleted` の行 1 つで、選んだアイテムは root: true、下のものは root: false
// (A4b-1)。すでに削除済みのものには書かない。アクティビティは「{ID} {タイトル} を削除」、下のものがあれば
// 「{ID} {タイトル} ほか N 件を削除」(A4b-8)。アイテムがない・削除済みなら port.ErrNotFound。
//
// アクティビティの N は保存の前に数える。数えてから保存するまでに同期で下のものが増減したときは、
// 書いた行の数と合わないことがある(保存は 1 つずつ順に行うので、利用者の操作どうしでは起きない)。
func (s Service) Delete(ctx context.Context, store port.StoreID, id item.ID) error {
	var summary string
	err := s.Stores.Read(ctx, store, func(r port.Reader) (err error) {
		summary, err = deleteSummary(r, id)
		return err
	})
	if err != nil {
		return err
	}
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: change.User(me), Summary: summary}
	return s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		_, below, err := liveBelow(tx, id)
		if err != nil {
			return err
		}
		return appendDeletes(tx.WriteItems(), id, below)
	})
}

// appendDeletes は、root に root: true、below に root: false の削除の行を書く。
func appendDeletes(w port.ItemWriter, root item.ID, below []item.Item) error {
	if err := w.Append(root, change.NewDeleteEntity(true)); err != nil {
		return err
	}
	for _, b := range below {
		if err := w.Append(b.ID, change.NewDeleteEntity(false)); err != nil {
			return err
		}
	}
	return nil
}

// deleteSummary は、アクティビティの 1 行(A4b-8)。タイトルは空白と改行を 1 つの空白にまとめる。
func deleteSummary(r port.Reader, id item.ID) (string, error) {
	it, below, err := liveBelow(r, id)
	if err != nil {
		return "", err
	}
	w, err := r.Workspaces().Get(it.Place.Workspace)
	if err != nil {
		return "", err
	}
	head := it.DisplayID(w.Prefix) + " " + item.NormalizeTitle(it.Title)
	if len(below) == 0 {
		return head + " を削除", nil
	}
	return fmt.Sprintf("%s ほか %d 件を削除", head, len(below)), nil
}

// BulkTarget は、一括削除で選んだアイテム 1 つ。
type BulkTarget struct {
	Store port.StoreID
	ID    item.ID
}

// BulkDeletePreview は、一括削除の確認ダイアログの中身(F2b1-10)。Stores は削除するアイテムのある Store の数、
// RetentionDays は Stores が 1 のときその Store の保持日数(ほかは 0)、ReadOnly は閲覧のみの Store の選んだ件数。
type BulkDeletePreview struct {
	Stores, RetentionDays, ReadOnly int
}

// BulkDeleted は、一括削除の結果(F2b1-12)。Deleted は書いた削除の行の数、ReadOnly は閲覧のみで飛ばした選んだ件数。
type BulkDeleted struct {
	Deleted, ReadOnly int
}

// rootDelete は、一括削除の 1 つの root と、その下の削除されていないもの。
type rootDelete struct {
	id    item.ID
	below []item.Item
}

// bulkPlan は、1 つの Store で選んだ ids から、書く root を作る。削除済み・存在しないものは飛ばす。
// ほかの選んだアイテムの下にあるものは root にしない(F2b1-7)。n は書く削除の行の数。
func bulkPlan(r port.Reader, ids []item.ID) (roots []rootDelete, n int, err error) {
	var all []rootDelete
	for _, id := range ids {
		if slices.ContainsFunc(all, func(d rootDelete) bool { return d.id == id }) {
			continue
		}
		_, below, err := liveBelow(r, id)
		if errors.Is(err, port.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, 0, err
		}
		all = append(all, rootDelete{id, below})
	}
	for _, d := range all {
		underOther := slices.ContainsFunc(all, func(o rootDelete) bool {
			return slices.ContainsFunc(o.below, func(b item.Item) bool { return b.ID == d.id })
		})
		if !underOther {
			roots = append(roots, d)
			n += 1 + len(d.below)
		}
	}
	return roots, n, nil
}

// bulkStore は、1 つの Store で削除する選んだアイテム。
type bulkStore struct {
	id  port.StoreID
	ids []item.ID
}

// bulkGroups は、targets を Store ごとに最初に出てきた順で束ね、閲覧のみの Store の選んだ件数を数える。
// 表示中でない Store を含むときは、何も読み書きせず port.ErrStoreNotFound(F2b1-6)。
func (s Service) bulkGroups(ctx context.Context, targets []BulkTarget) (groups []bulkStore, readOnly int, err error) {
	infos, err := s.StoreList.List(ctx)
	if err != nil {
		return nil, 0, err
	}
	for _, t := range targets {
		i := slices.IndexFunc(infos, func(x port.StoreInfo) bool { return x.ID == t.Store })
		if i < 0 {
			return nil, 0, port.ErrStoreNotFound
		}
		if infos[i].ReadOnly {
			readOnly++
			continue
		}
		g := slices.IndexFunc(groups, func(x bulkStore) bool { return x.id == t.Store })
		if g < 0 {
			groups = append(groups, bulkStore{id: t.Store})
			g = len(groups) - 1
		}
		groups[g].ids = append(groups[g].ids, t.ID)
	}
	return groups, readOnly, nil
}

// BulkDeletePreview は、targets を一括削除したときの確認ダイアログの中身を返す。
// 表示中でない Store を含むときは port.ErrStoreNotFound。
func (s Service) BulkDeletePreview(ctx context.Context, targets []BulkTarget) (BulkDeletePreview, error) {
	groups, readOnly, err := s.bulkGroups(ctx, targets)
	if err != nil {
		return BulkDeletePreview{}, err
	}
	out := BulkDeletePreview{ReadOnly: readOnly}
	retention := 0
	for _, g := range groups {
		err := s.Stores.Read(ctx, g.id, func(r port.Reader) error {
			_, n, err := bulkPlan(r, g.ids)
			if err != nil || n == 0 {
				return err
			}
			out.Stores++
			retention, err = r.RetentionDays()
			return err
		})
		if err != nil {
			return BulkDeletePreview{}, err
		}
	}
	if out.Stores == 1 {
		out.RetentionDays = retention
	}
	return out, nil
}

// BulkDelete は、targets を Store ごとに最初に出てきた順で 1 回ずつ保存して削除する(F2b1-6〜9)。
// 閲覧のみの Store のものは書かずに ReadOnly に数える。削除済み・存在しないものは飛ばし、書く行がない Store は
// 保存しない。アクティビティは「{N} 件を一括削除」。N は保存の前に数える(Delete と同じ。F2b1-8)。
// Store の保存に失敗したらそこでやめて、エラーを返す(それまでの Store は保存済み)。
func (s Service) BulkDelete(ctx context.Context, targets []BulkTarget) (BulkDeleted, error) {
	groups, readOnly, err := s.bulkGroups(ctx, targets)
	if err != nil {
		return BulkDeleted{}, err
	}
	out := BulkDeleted{ReadOnly: readOnly}
	for _, g := range groups {
		n, err := s.bulkDeleteStore(ctx, g)
		if err != nil {
			return out, err
		}
		out.Deleted += n
	}
	return out, nil
}

// bulkDeleteStore は、1 つの Store の分を 1 回の保存で削除し、書いた行の数を返す。
func (s Service) bulkDeleteStore(ctx context.Context, g bulkStore) (int, error) {
	var n int
	err := s.Stores.Read(ctx, g.id, func(r port.Reader) (err error) {
		_, n, err = bulkPlan(r, g.ids)
		return err
	})
	if err != nil || n == 0 {
		return 0, err
	}
	me, err := s.Identity.Me(ctx, g.id)
	if err != nil {
		return 0, err
	}
	op := port.Operation{Actor: change.User(me), Summary: fmt.Sprintf("%d 件を一括削除", n)}
	err = s.Stores.Write(ctx, g.id, op, func(tx port.Tx) error {
		roots, _, err := bulkPlan(tx, g.ids)
		if err != nil {
			return err
		}
		for _, d := range roots {
			if err := appendDeletes(tx.WriteItems(), d.id, d.below); err != nil {
				return err
			}
		}
		return nil
	})
	return n, err
}
