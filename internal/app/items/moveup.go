package items

import (
	"context"
	"errors"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// MoveUp は、同じ親の兄弟の中で、直前の兄弟と並び順の値を入れ替える(02「子と Step」の「上へ」、04 の
// 「上へ移動」)。先頭なら何も保存しない。アクティビティは「{ID} の順位を変更」。
// 直前の兄弟と並び順の値が同じとき(同期で重なったとき)は、入れ替えても並びが変わらないので何も保存しない。
// 重なりの修復はトラック C。
func (s Service) MoveUp(ctx context.Context, store port.StoreID, id item.ID) error {
	_, display, err := s.withID(ctx, store, id)
	if err != nil {
		return err
	}
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: change.User(me), Summary: fmt.Sprintf("%s の順位を変更", display)}
	return s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		it, err := tx.Items().Get(id)
		if err != nil {
			return err
		}
		all, err := tx.Items().List(it.Place.Project)
		if err != nil {
			return err
		}
		sib := siblings(all, it.Place.Parent)
		item.SortSiblings(sib)
		i := indexOf(sib, id)
		if i <= 0 || sib[i-1].Rank == it.Rank {
			return nil // 行のない Write は何も残さない
		}
		prev := sib[i-1]
		return errors.Join(
			tx.WriteItems().Append(it.ID, change.NewSet("rank", it.Rank, prev.Rank)),
			tx.WriteItems().Append(prev.ID, change.NewSet("rank", prev.Rank, it.Rank)),
		)
	})
}

// siblings は、parent の子(parent が nil ならプロジェクトの直下)を返す。
func siblings(all []item.Item, parent *item.ID) []item.Item {
	var out []item.Item
	for _, it := range all {
		if (parent == nil && it.Place.Parent == nil) ||
			(parent != nil && it.Place.Parent != nil && *it.Place.Parent == *parent) {
			out = append(out, it)
		}
	}
	return out
}

func indexOf(its []item.Item, id item.ID) int {
	for i, it := range its {
		if it.ID == id {
			return i
		}
	}
	return -1
}
