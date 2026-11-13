package transfer_test

import (
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

func ids(its []item.Item) []item.ID {
	out := make([]item.ID, 0, len(its))
	for _, it := range its {
		out = append(out, it.ID)
	}
	return out
}

// projectItems は、プロジェクト P のアイテム。Epic E1(順位 b)・E2(順位 a)・削除済みの E3(順位 0)、
// E2 の下の T1 と削除済みの T2、E1 の下の T3。
func projectItems() []item.Item {
	var all []item.Item
	for _, it := range []item.Item{
		node("E1", "", "b", false), node("E2", "", "a", false), node("E3", "", "0", true),
		node("T1", "E2", "a", false), node("T2", "E2", "0", true), node("T3", "E1", "a", false),
	} {
		it.Place.Project = "P"
		all = append(all, it)
	}
	return all
}

// 02 移動: Epic は削除されていないものを順位の順 → 削除済みを順位の順。それぞれ Epic → その下の順。
func TestCollectProjectOrdersEpicsLiveThenDeleted(t *testing.T) {
	s := transfer.CollectProject("P", projectItems())
	want := []item.ID{"E2", "T1", "T2", "E1", "T3", "E3"}
	if got := ids(s.Items()); !reflect.DeepEqual(got, want) {
		t.Errorf("Items = %v, want %v", got, want)
	}
	if !s.Contains("T3") || s.Contains("X") {
		t.Errorf("Contains が合わない")
	}
}

// 外との関連は、集まり全体の外(all にいて集まりにいないもの)だけ。Epic どうしの間は外ではない。
func TestCollectProjectCutsOnlyOutside(t *testing.T) {
	all := projectItems()
	for i := range all {
		if all[i].ID == "T1" {
			all[i] = related(related(all[i], "blocks", "X"), "blocks", "T3")
		}
	}
	all = append(all, related(node("X", "", "z", false), "relates", "T1"), related(node("Y", "", "y", false), "blocks", "NOWHERE"))

	s := transfer.CollectProject("P", all)
	want := []transfer.Cut{{From: "T1", To: "X", Type: "blocks"}, {From: "X", To: "T1", Type: "relates"}}
	if !reflect.DeepEqual(s.Outside, want) {
		t.Errorf("Outside = %v, want %v", s.Outside, want)
	}
	for _, e := range s.Epics {
		if len(e.Outside) != 0 {
			t.Errorf("Epic ごとの Outside は空: %v", e.Outside)
		}
	}
}

func TestCollectProjectWithoutItems(t *testing.T) {
	other := node("Z", "", "a", false)
	other.Place.Project = "OTHER"
	s := transfer.CollectProject("P", []item.Item{other})
	if len(s.Epics) != 0 || len(s.Items()) != 0 || len(s.Outside) != 0 {
		t.Errorf("空のはず: %+v", s)
	}
}

func TestNumberProjectChainsEpics(t *testing.T) {
	got := transfer.NumberProject(transfer.CollectProject("P", projectItems()), 13)
	want := map[item.ID]int{"E2": 13, "T1": 14, "T2": 15, "E1": 16, "T3": 17, "E3": 18}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("NumberProject = %v, want %v", got, want)
	}
}
