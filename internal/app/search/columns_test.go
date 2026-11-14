package search

import (
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// addWorkspace は、Store store にワークスペースを足す。edit で既定の設定を変える。
func addWorkspace(t *testing.T, f fixture, store port.StoreID, id workspace.ID, rank string, edit func(*workspace.Settings)) {
	t.Helper()
	st := workspace.DefaultSettings(func() change.ID { return "01TESTVAL00000000000000000" })
	if edit != nil {
		edit(&st)
	}
	ws := workspace.Workspace{ID: id, Name: string(id), Prefix: "W" + string(id[len(id)-1])}
	d, err := workspace.CreateWithRank(ws, st, rank)
	must(t, err)
	op := port.Operation{Actor: change.User("taro"), Summary: "テスト"}
	must(t, f.stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(id, d, numbering.Draft(numbering.Projects, 1))
	}))
}

func withProp(key, name string, typ workspace.PropertyType) func(*workspace.Settings) {
	return func(s *workspace.Settings) {
		s.Properties = append(s.Properties, workspace.Property{
			ID: change.ID("01TESTPROP" + key), Key: key, Name: name, Type: typ, Kinds: workspace.Kinds, Rank: "z",
		})
	}
}

func rename(key, name string) func(*workspace.Settings) {
	return func(s *workspace.Settings) {
		for i := range s.Properties {
			if s.Properties[i].Key == key {
				s.Properties[i].Name = name
			}
		}
	}
}

func columnsFor(t *testing.T, f fixture) []Column {
	t.Helper()
	_, cols, err := f.svc.catalog(t.Context())
	must(t, err)
	return cols
}

func columnByKey(t *testing.T, cols []Column, key string) Column {
	t.Helper()
	for _, c := range cols {
		if c.Key == key {
			return c
		}
	}
	t.Fatalf("列 %s がない: %+v", key, cols)
	return Column{}
}

func TestColumnsFixedOrder(t *testing.T) {
	f := newFixture(t)
	must(t, apptest.Sample(t.Context(), f.stores, store1))
	cols := columnsFor(t, f)
	keys := []string{"id", "kind", "title", "status", "assignee", "priority", "start", "end", "points", "estimate", "system", "sprint", "release", "project", "parent", "created", "updated"}
	names := []string{"ID", "種類", "タイトル", "ステータス", "担当者", "優先度", "開始日", "終了日", "ポイント", "見積もり時間", "関連システム", "スプリント", "リリース", "プロジェクト", "親", "作成日", "更新日"}
	if len(cols) != len(keys) {
		t.Fatalf("列の数 = %d, want %d", len(cols), len(keys))
	}
	for i, c := range cols {
		if c.Key != keys[i] || c.Name != names[i] || !c.Sortable || c.Type != "" {
			t.Errorf("cols[%d] = %+v, want %s %s", i, c, keys[i], names[i])
		}
	}
	if got := columnByKey(t, cols, "system").Builtin; got != "systems" {
		t.Errorf("system の Builtin = %q", got)
	}
	if got := columnByKey(t, cols, "title").Builtin; got != "" {
		t.Errorf("title の Builtin = %q", got)
	}
}

func TestColumnsBuiltinNames(t *testing.T) {
	f := newFixture(t)
	addWorkspace(t, f, store1, "01TESTWS00000000000000001", "a", rename("points", "SP"))
	addWorkspace(t, f, store2, "01TESTWS00000000000000002", "a", rename("points", "SP"))
	if got := columnByKey(t, columnsFor(t, f), "points").Name; got != "SP" {
		t.Errorf("すべて改名: %q", got)
	}
	f = newFixture(t)
	addWorkspace(t, f, store1, "01TESTWS00000000000000001", "a", rename("points", "SP"))
	addWorkspace(t, f, store2, "01TESTWS00000000000000002", "a", nil)
	if got := columnByKey(t, columnsFor(t, f), "points").Name; got != "ポイント" {
		t.Errorf("1 つだけ改名: %q", got)
	}
}

func TestColumnsCustomMerged(t *testing.T) {
	f := newFixture(t)
	addWorkspace(t, f, store1, "01TESTWS0000000000000000A", "a", withProp("customer", "顧客", workspace.TypeText))
	addWorkspace(t, f, store2, "01TESTWS0000000000000000B", "a", withProp("Customer", "お客様", workspace.TypeText))
	cols := columnsFor(t, f)
	if len(cols) != 18 {
		t.Fatalf("列の数 = %d, want 18", len(cols))
	}
	want := Column{Key: "p:customer", Name: "customer", Type: workspace.TypeText, Sortable: true}
	if got := cols[17]; got != want {
		t.Errorf("cols[17] = %+v, want %+v", got, want)
	}
}

func TestColumnsCustomSameNameKept(t *testing.T) {
	f := newFixture(t)
	addWorkspace(t, f, store1, "01TESTWS0000000000000000A", "a", withProp("customer", "顧客", workspace.TypeText))
	addWorkspace(t, f, store2, "01TESTWS0000000000000000B", "a", withProp("Customer", "顧客", workspace.TypeText))
	if got := columnByKey(t, columnsFor(t, f), "p:customer").Name; got != "顧客" {
		t.Errorf("名前 = %q", got)
	}
}

func TestColumnsSameKeyDifferentType(t *testing.T) {
	f := newFixture(t)
	addWorkspace(t, f, store1, "01TESTWS0000000000000000A", "a", withProp("due", "期日", workspace.TypeDate))
	addWorkspace(t, f, store2, "01TESTWS0000000000000000B", "a", withProp("due", "期日", workspace.TypeText))
	cols := columnsFor(t, f)
	if len(cols) != 18 || cols[17].Type != workspace.TypeDate {
		t.Errorf("cols = %+v", cols[17:])
	}
}

func TestColumnsBuiltinSpelling(t *testing.T) {
	f := newFixture(t)
	addWorkspace(t, f, store1, "01TESTWS0000000000000000A", "a", func(s *workspace.Settings) {
		withProp("status", "別のステータス", workspace.TypeText)(s)
		withProp("title", "別のタイトル", workspace.TypeText)(s)
	})
	cols := columnsFor(t, f)
	if c := columnByKey(t, cols, "p:status"); c.Sortable {
		t.Errorf("p:status は並べ替えられない: %+v", c)
	}
	if c := columnByKey(t, cols, "p:title"); c.Sortable {
		t.Errorf("p:title は並べ替えられない(固定の列の鍵と同じ綴り): %+v", c)
	}
	if !columnByKey(t, cols, "status").Sortable {
		t.Errorf("固定の status は並べ替えられる")
	}
}

func TestColumnsHiddenStoreLeftOut(t *testing.T) {
	f := newFixture(t)
	addWorkspace(t, f, store3, "01TESTWS0000000000000000C", "a", withProp("customer", "顧客", workspace.TypeText))
	if cols := columnsFor(t, f); len(cols) != 17 {
		t.Errorf("非表示の Store の列が出ている: %+v", cols[17:])
	}
	keys, _, err := f.svc.catalog(t.Context())
	must(t, err)
	if !keys["customer"] {
		t.Errorf("キーは知っているはず: %v", keys)
	}
}

func TestColumnsHiddenPropertyKept(t *testing.T) {
	f := newFixture(t)
	addWorkspace(t, f, store1, "01TESTWS0000000000000000A", "a", func(s *workspace.Settings) {
		for i := range s.Properties {
			if s.Properties[i].Key == "priority" {
				s.Properties[i].Hidden = true
			}
		}
	})
	if got := columnByKey(t, columnsFor(t, f), "priority"); got.Name != "優先度" {
		t.Errorf("priority = %+v", got)
	}
}

func TestColumnsWorkspaceOrderByRank(t *testing.T) {
	f := newFixture(t)
	addWorkspace(t, f, store1, "01TESTWS0000000000000000A", "b", withProp("one", "1", workspace.TypeText))
	addWorkspace(t, f, store1, "01TESTWS0000000000000000B", "a", withProp("two", "2", workspace.TypeText))
	cols := columnsFor(t, f)
	if got := []string{cols[17].Key, cols[18].Key}; !reflect.DeepEqual(got, []string{"p:two", "p:one"}) {
		t.Errorf("追加の列 = %v", got)
	}
}
