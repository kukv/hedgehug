package items

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/rank"
)

// Reorder は、id を、同じ親の兄弟 anchor の直前(after なら直後)に置く(02「子と Step」の「上へ」、
// 「上から順の構成」の上へ移動・下へ移動、04「ツリー」の「行」。A4b-9)。
// 書くのは、動かした 1 件の `set rank` だけで、値は `rank.Around` で作る(前後の兄弟の値の間。同じ順位の群の
// 中には入らず、群の手前(after なら後ろ)に入る。F2a-2 と同じ規則)。
// どの兄弟を基準にするかは画面が決める(絞り込み中は、見えている兄弟)。基準と置く場所の間にいる
// 見えていない兄弟は動かさない。削除済みの兄弟は、いないものとして数える。
// アクティビティは「{ID} の順位を変更」。
//
// anchor が、id と同じ親の削除されていない別の兄弟でなければ port.ErrNotFound。すでにその位置なら何も保存しない。
func (s Service) Reorder(ctx context.Context, store port.StoreID, id, anchor item.ID, after bool) error {
	_, display, err := s.editableID(ctx, store, id)
	if err != nil {
		return err
	}
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: change.User(me), Summary: fmt.Sprintf("%s の順位を変更", display)}
	return s.Stores.Write(ctx, store, op, func(tx port.Tx) error { return reorder(ctx, tx, id, anchor, after) })
}

// reorder は、tx の中で、削除済み(root: true)なら削除を取り消してから、順位を書く。
func reorder(ctx context.Context, tx port.Tx, id, anchor item.ID, after bool) error {
	was, err := port.EditableItem(tx, id)
	if err != nil {
		return err
	}
	if err := port.ReviveItem(ctx, tx, id); err != nil {
		return err
	}
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
	r, ok := gap(sib, i, j, after)
	if !ok {
		return port.Unchanged(was.Deleted) // 行のない Write は何も残さない
	}
	return tx.WriteItems().Append(id, change.NewSet("rank", it.Rank, r))
}

// gap は、sib の i 番目を j 番目(基準)の直前(after なら直後)に置くときの順位を、ほかの兄弟の並びから
// rank.Around で作る。すでにその位置のときは ok が false。
func gap(sib []item.Item, i, j int, after bool) (string, bool) {
	if (after && j == i-1) || (!after && j == i+1) {
		return "", false
	}
	var ranks []string
	for k, it := range sib {
		if k != i {
			ranks = append(ranks, it.Rank)
		}
	}
	if i < j {
		j-- // 動かすものを除くと、基準が 1 つ前に詰まる
	}
	return rank.Around(ranks, j, after), true
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
