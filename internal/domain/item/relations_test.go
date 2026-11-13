package item_test

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

func TestFromLogReadsRelations(t *testing.T) {
	it := item.Item{ID: "A", Place: item.Place{Workspace: "W", Project: "P", Number: 1}, Kind: item.Task, Rank: "V", Title: "a"}
	meta := change.Meta{Op: "o", At: time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC), Actor: change.User("taro")}
	n := 0
	newID := func() change.ID { n++; return change.ID(fmt.Sprintf("R%02d", n)) }
	log := change.Stamp(nil, []change.Draft{item.Create(it)}, meta, newID)
	log = append(log, change.Stamp(log, []change.Draft{
		change.NewAdd("relations", item.Relation{Type: "blocks", To: "B"}),
		change.NewAdd("relations", item.Relation{Type: "relates", To: "C"}),
		change.NewAdd("relations", map[string]any{"type": "blocks", "to": "bad id!"}), // 内部 ID でない相手は読み飛ばす
		change.NewRemove("relations", item.Relation{Type: "relates", To: "C"}),
	}, meta, newID)...)
	got, ok := item.FromLog(log)
	if !ok {
		t.Fatal("no item")
	}
	if want := []item.Relation{{Type: "blocks", To: "B"}}; !slices.Equal(got.Relations, want) {
		t.Errorf("Relations = %+v, want %+v", got.Relations, want)
	}
	if got.Blocked {
		t.Error("Blocked は保存しない値なので、FromLog では false")
	}
}
