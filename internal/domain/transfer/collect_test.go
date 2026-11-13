package transfer_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

func related(it item.Item, typ string, to change.ID) item.Item {
	it.Relations = append(it.Relations, item.Relation{Type: typ, To: to})
	return it
}

func TestCollectBelowAndOutside(t *testing.T) {
	e := node("E", "", "a", false)
	a := related(node("A", "E", "a", false), "relates", "B") // 中どうし: 残る
	b := related(node("B", "E", "b", true), "relates", "Y")  // 外の削除済みへ
	step := node("T", "A", "a", false)
	step.Kind = item.Step
	x := related(node("X", "", "z", false), "blocks", "A") // 外から中へ
	y := node("Y", "", "y", true)
	z := related(node("Z", "", "y", false), "blocks", "NOWHERE") // 壊れた関連は数えない
	all := []item.Item{y, x, b, step, a, e, z}

	set, err := transfer.Collect("E", all)
	if err != nil {
		t.Fatal(err)
	}
	var below []item.ID
	for _, it := range set.Below {
		below = append(below, it.ID)
	}
	if set.Root.ID != "E" || !slices.Equal(below, []item.ID{"A", "T", "B"}) {
		t.Errorf("root %s, below %v (削除済みも含み、深さ優先・兄弟の順)", set.Root.ID, below)
	}
	want := []transfer.Cut{{From: "B", To: "Y", Type: "relates"}, {From: "X", To: "A", Type: "blocks"}}
	if !slices.Equal(set.Outside, want) {
		t.Errorf("Outside = %+v, want %+v (両方の側。From・To・Type の順)", set.Outside, want)
	}
	if !set.Contains("T") || !set.Contains("E") || set.Contains("X") {
		t.Error("Contains が中と外を分けていない")
	}
}

func TestCollectRefuses(t *testing.T) {
	step := node("T", "A", "a", false)
	step.Kind = item.Step
	all := []item.Item{node("A", "", "a", false), step}
	if _, err := transfer.Collect("T", all); !errors.Is(err, transfer.ErrStep) {
		t.Errorf("step root = %v, want ErrStep", err)
	}
	if _, err := transfer.Collect("NONE", all); !errors.Is(err, transfer.ErrNotFound) {
		t.Errorf("unknown root = %v, want ErrNotFound", err)
	}
}
