package item_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

const (
	sTodo   change.ID = "01STATUS00000000000000000A"
	sDoing  change.ID = "01STATUS00000000000000000B"
	pMedium change.ID = "01PRIO0000000000000000000A"
	yWeb    change.ID = "01SYS00000000000000000000A"
	yAPI    change.ID = "01SYS00000000000000000000B"
	fEnv    change.ID = "01PROP0000000000000000000A" // 単一選択
	fTags   change.ID = "01PROP0000000000000000000B" // 複数選択
	vProd   change.ID = "01CHOICE00000000000000000A"
	vA      change.ID = "01CHOICE00000000000000000B"
	vB      change.ID = "01CHOICE00000000000000000C"
)

// rows は、drafts を 1 行ずつ別の操作として並べる(prev は Stamp が付ける)。
func rows(drafts ...change.Draft) []change.Record {
	var log []change.Record
	at := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	for i, d := range drafts {
		id := change.ID(fmt.Sprintf("01REC%021d", i))
		meta := change.Meta{Op: id, At: at.Add(time.Duration(i) * time.Second), Actor: change.User("taro")}
		log = append(log, change.Stamp(log, []change.Draft{d}, meta, func() change.ID { return id })...)
	}
	return log
}

func TestValuesFromLog(t *testing.T) {
	base := item.Item{ID: "01ITEM0000000000000000000A", Kind: item.Task, Title: "x"}
	log := rows(
		item.Create(base),
		change.NewSet("status", nil, sTodo),
		change.NewSet("status", sTodo, sDoing),
		change.NewSet("priority", nil, pMedium),
		change.NewAdd("systems", yWeb),
		change.NewAdd("systems", yAPI),
		change.NewRemove("systems", yWeb),
		change.NewSet(item.FieldKey(fEnv), nil, vProd),
		change.NewAdd(item.FieldKey(fTags), vA),
		change.NewAdd(item.FieldKey(fTags), vB),
	)
	it, ok := item.FromLog(log)
	if !ok {
		t.Fatal("not created")
	}
	if it.Status != sDoing || it.Priority != pMedium || !reflect.DeepEqual(it.Systems, []change.ID{yAPI}) {
		t.Errorf("item = %+v", it)
	}
	want := map[change.ID][]change.ID{fEnv: {vProd}, fTags: {vA, vB}}
	if !reflect.DeepEqual(it.Choices, want) {
		t.Errorf("choices = %v, want %v", it.Choices, want)
	}
	if item.MultiChoice(log, fEnv) || !item.MultiChoice(log, fTags) {
		t.Error("MultiChoice: 単一は false、複数は true")
	}
}

// 値のないアイテムは、どれも空。単一選択を null にしたら、選択肢から消える。
func TestValuesCleared(t *testing.T) {
	base := item.Item{ID: "01ITEM0000000000000000000A", Kind: item.Task, Title: "x"}
	it, _ := item.FromLog(rows(item.Create(base)))
	if it.Status != "" || it.Priority != "" || it.Systems != nil || it.Choices != nil {
		t.Errorf("empty item = %+v", it)
	}
	it, _ = item.FromLog(rows(
		item.Create(base),
		change.NewSet(item.FieldKey(fEnv), nil, vProd),
		change.NewSet(item.FieldKey(fEnv), vProd, nil),
	))
	if it.Choices != nil {
		t.Errorf("choices = %v", it.Choices)
	}
}

// 作成の行に配列で書いた fields は、複数選択として読む(A3 が作成で書く形)。
func TestChoicesInCreate(t *testing.T) {
	log := rows(change.NewCreate(map[string]any{
		"id": "01ITEM0000000000000000000A", "kind": "Task", "title": "x",
		"status": sTodo, "systems": []change.ID{yWeb},
		item.FieldKey(fEnv): vProd, item.FieldKey(fTags): []change.ID{vA},
	}))
	it, _ := item.FromLog(log)
	want := map[change.ID][]change.ID{fEnv: {vProd}, fTags: {vA}}
	if it.Status != sTodo || !reflect.DeepEqual(it.Systems, []change.ID{yWeb}) || !reflect.DeepEqual(it.Choices, want) {
		t.Errorf("item = %+v", it)
	}
}
