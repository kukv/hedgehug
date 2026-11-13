package transfer_test

import (
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

// node は、テスト用のアイテム。parent が空なら親なし。
func node(id, parent, rank string, deleted bool) item.Item {
	it := item.Item{ID: change.ID(id), Rank: rank, Created: item.Stamp{At: at}}
	if parent != "" {
		p := change.ID(parent)
		it.Place.Parent = &p
	}
	if deleted {
		it.Deleted = &change.Deletion{At: at}
	}
	return it
}

// 02「移すもの」: 移すアイテムが先、続いて子・孫を並び順に深さ優先。削除済みの子は、同じ親の下の削除されていない子のあと。
func TestNumberOrder(t *testing.T) {
	root := node("E", "", "a", false)
	below := []item.Item{ // わざと並びを崩す
		node("B", "E", "b", true),
		node("A2", "A", "0", true), // 順位は A1 より前だが、削除済みなので A1 のあと
		node("C", "E", "c", false),
		node("A1", "A", "a", false),
		node("A", "E", "a", false),
	}
	got := transfer.Number(root, below, 13)
	want := map[item.ID]int{"E": 13, "A": 14, "A1": 15, "A2": 16, "C": 17, "B": 18}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Number = %v, want %v", got, want)
	}
}

func TestNumberOfAnItemWithoutChildren(t *testing.T) {
	got := transfer.Number(node("T", "E", "a", false), nil, 7)
	if !reflect.DeepEqual(got, map[item.ID]int{"T": 7}) {
		t.Errorf("Number = %v", got)
	}
}
