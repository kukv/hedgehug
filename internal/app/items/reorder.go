package items

import (
	"context"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/rank"
)

// Reorder は、id を、同じ親の兄弟 anchor の直前(after なら直後)に置く(02「子と Step」の「上へ」、
// 「上から順の構成」の上へ移動・下へ移動、04「ツリー」の「行」。A4b-9)。
// 書くのは、動かした 1 件の `set rank` だけで、値は前後の兄弟の値の間(`rank.Between`。端は `rank.After`)で作る。
// どの兄弟を基準にするかは画面が決める(絞り込み中は、見えている兄弟)。基準と置く場所の間にいる
// 見えていない兄弟は動かさない。削除済みの兄弟は、いないものとして数える。
// アクティビティは「{ID} の順位を変更」。
//
// anchor が、id と同じ親の削除されていない別の兄弟でなければ port.ErrNotFound。すでにその位置なら何も保存しない。
// 前後の兄弟の順位が同じで間の値が作れないとき(同期で重なったとき)も何も保存しない。重なりの修復はトラック C。
func (s Service) Reorder(ctx context.Context, store port.StoreID, id, anchor item.ID, after bool) error {
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
		it, err := port.LiveItem(tx, id)
		if err != nil {
			return err
		}
		if id == anchor {
			return port.ErrNotFound
		}
		all, err := port.LiveItems(tx, it.Place.Project)
		if err != nil {
			return err
		}
		sib := siblings(all, it.Place.Parent)
		item.SortSiblings(sib)
		i, j := indexOf(sib, id), indexOf(sib, anchor)
		if i < 0 || j < 0 {
			return port.ErrNotFound
		}
		lo, hi, ok := gap(sib, i, j, after)
		if !ok {
			return nil // 行のない Write は何も残さない
		}
		return tx.WriteItems().Append(id, change.NewSet("rank", it.Rank, rank.Between(lo, hi)))
	})
}

// gap は、sib の i 番目を j 番目(基準)の直前(after なら直後)に置くときの、間に入る値の下と上を返す。
// 上が空なら、末尾(rank.Between は下より後ろの値を返す)。すでにその位置のとき、
// または前後の値が同じで間の値が作れないときは ok が false。
func gap(sib []item.Item, i, j int, after bool) (lo, hi string, ok bool) {
	if (after && j == i-1) || (!after && j == i+1) {
		return "", "", false
	}
	rest := slices.Delete(slices.Clone(sib), i, i+1)
	if i < j {
		j-- // 動かすものを除くと、基準が 1 つ前に詰まる
	}
	if after {
		lo = rest[j].Rank
		if j+1 < len(rest) {
			hi = rest[j+1].Rank
		}
		return lo, hi, hi == "" || lo < hi
	}
	hi = rest[j].Rank
	if j > 0 {
		lo = rest[j-1].Rank
	}
	return lo, hi, lo < hi
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
