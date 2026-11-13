package items

import (
	"context"
	"fmt"

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
