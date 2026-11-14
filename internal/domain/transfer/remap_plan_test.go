package transfer_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

// 別のワークスペース・別の Store へ移すときは、スプリントとリリースを set で外す(02「移動先で変わること」、
// 保存の設計「移動・番号・旧 ID」)。行はプロパティのあと、ウォッチの前。
func TestRemapUnsetsSprintAndRelease(t *testing.T) {
	from, to := pair()
	it := item.Item{ID: "I1", Sprint: "SP1", Release: "RL1", Watchers: []string{"Hanako"}}
	for name, people := range map[string]transfer.PeopleMap{"same store": nil, "other store": {"hanako": "hanako2"}} {
		got := show(transfer.Remap(it, from, to, none, people, nil))
		i, j := slices.Index(got, `set sprint "SP1">null`), slices.Index(got, `set release "RL1">null`)
		if i < 0 || j != i+1 {
			t.Errorf("%s: rows = %q, want the sprint and then the release unset", name, got)
		}
		if people != nil && got[len(got)-1] != `add watchers >"hanako2"` {
			t.Errorf("%s: last row = %s, want the watchers", name, got[len(got)-1])
		}
	}
}

// スプリントにもリリースにも入っていなければ、行を書かない。外したことを空になった項目には並べない(ダイアログで知らせ済み)。
func TestRemapWithoutPlanWritesNoPlanRows(t *testing.T) {
	from, to := pair()
	got := transfer.Remap(item.Item{ID: "I1"}, from, to, none, nil, nil)
	if len(got.Drafts) != 0 || len(got.Emptied) != 0 {
		t.Errorf("Remap = %q / %v, want no rows", show(got), got.Emptied)
	}
	got = transfer.Remap(item.Item{ID: "I1", Sprint: "SP1"}, from, to, none, nil, nil)
	if len(got.Emptied) != 0 {
		t.Errorf("Emptied = %v, want none", got.Emptied)
	}
}
