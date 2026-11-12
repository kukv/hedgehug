package item_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/item"
)

func TestSortSiblings(t *testing.T) {
	at := func(m int) item.Stamp { return item.Stamp{At: time.Date(2026, 11, 13, 9, m, 0, 0, time.UTC)} }
	its := []item.Item{
		{ID: "01C", Rank: "W", Created: at(0)},
		{ID: "01B", Rank: "V", Created: at(1)},
		{ID: "01D", Rank: "V", Created: at(0)},
		{ID: "01A", Rank: "V", Created: at(1)},
	}
	item.SortSiblings(its)
	// 並び順の値 V が先、V どうしは作成の早い 01D、同じ時刻の 01A と 01B は内部 ID の順、最後に W。
	want := []item.ID{"01D", "01A", "01B", "01C"}
	for i, id := range want {
		if its[i].ID != id {
			t.Fatalf("order = %v, want %v", its, want)
		}
	}
}
