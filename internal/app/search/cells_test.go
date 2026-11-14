package search

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/query"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const gone change.ID = "01GONE00000000000000000000"

func defaults() workspace.Settings {
	n := 0
	return workspace.DefaultSettings(func() change.ID {
		n++
		return change.ID("01CELLVAL0000000000000000" + string(rune('A'+n)))
	})
}

func custom(key string, typ workspace.PropertyType, kinds ...string) workspace.Property {
	if kinds == nil {
		kinds = workspace.Kinds
	}
	return workspace.Property{ID: change.ID("01CELLPROP" + key), Key: key, Name: key, Type: typ, Kinds: kinds, Rank: "z"}
}

func customCol(key string, typ workspace.PropertyType) Column {
	return Column{Key: "p:" + key, Name: key, Type: typ}
}

func cellFacts(st workspace.Settings, it item.Item) query.Facts {
	return query.Facts{Settings: st, Item: it, Names: map[string]string{"tanaka": "田中"}}
}

func dropKind(st *workspace.Settings, key, kind string) {
	for i, p := range st.Properties {
		if p.Key == key {
			st.Properties[i].Kinds = slices.DeleteFunc(slices.Clone(p.Kinds), func(k string) bool { return k == kind })
		}
	}
}

func TestCellsUnusedByKind(t *testing.T) {
	st := defaults()
	dropKind(&st, "priority", "Step")
	cols := columnsOf([]*space{{settings: st}})
	for _, c := range []struct {
		kind item.Kind
		want []string
	}{
		{item.Step, []string{"priority", "sprint", "release"}},
		{item.Epic, []string{"sprint"}},
		{item.Story, nil},
		{item.Task, nil},
	} {
		got := cellsOf(cellFacts(st, item.Item{Kind: c.kind}), cols).Unused
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: Unused = %v, want %v", c.kind, got, c.want)
		}
	}
}

func TestCellsUnusedMissingProperty(t *testing.T) {
	st := defaults()
	col := []Column{customCol("customer", workspace.TypeText)}
	cells := cellsOf(cellFacts(st, item.Item{Kind: item.Task}), col)
	if !reflect.DeepEqual(cells.Unused, []string{"p:customer"}) {
		t.Errorf("Unused = %v", cells.Unused)
	}
	if _, ok := cells.Custom["p:customer"]; ok {
		t.Error("Custom に入っている")
	}
	st.Properties = append(st.Properties, custom("customer", workspace.TypeText, "Story"))
	cells = cellsOf(cellFacts(st, item.Item{Kind: item.Task}), col)
	if !reflect.DeepEqual(cells.Unused, []string{"p:customer"}) {
		t.Errorf("Kinds にない: Unused = %v", cells.Unused)
	}
}

func TestCellsRemoved(t *testing.T) {
	st := defaults()
	tags := custom("tags", workspace.TypeMulti)
	tags.Choices = []workspace.Value{{ID: "01X", Name: "x"}}
	st.Properties = append(st.Properties, tags)
	it := item.Item{
		Kind: item.Task, Priority: gone, Systems: []change.ID{st.Systems[0].ID, gone},
		Choices: map[change.ID][]change.ID{tags.ID: {"01X", gone}},
	}
	cols := columnsOf([]*space{{settings: st}})
	cells := cellsOf(cellFacts(st, it), cols)
	if want := []string{"priority", "system", "p:tags"}; !reflect.DeepEqual(cells.Removed, want) {
		t.Errorf("Removed = %v, want %v", cells.Removed, want)
	}
	if want := []string{"Web", "(削除された値)"}; !reflect.DeepEqual(cells.Systems, want) {
		t.Errorf("Systems = %v, want %v", cells.Systems, want)
	}
}

// ステータスの ID と単一選択の ID が一覧にないときも「(削除された値)」にする。名前がちょうど「(削除された値)」の
// 関連システムは、一覧にある値なので Removed に入れない。
func TestCellsRemovedStatusSingleAndNamedSystem(t *testing.T) {
	st := defaults()
	cust := custom("cust", workspace.TypeSingle)
	cust.Choices = []workspace.Value{{ID: "01A", Name: "A社"}}
	st.Properties = append(st.Properties, cust)
	it := item.Item{Kind: item.Task, Status: gone, Choices: map[change.ID][]change.ID{cust.ID: {gone}}}
	cells := cellsOf(cellFacts(st, it), columnsOf([]*space{{settings: st}}))
	if want := []string{"status", "p:cust"}; !reflect.DeepEqual(cells.Removed, want) {
		t.Errorf("Removed = %v, want %v", cells.Removed, want)
	}
	if got := cells.Custom["p:cust"].Text; got != "(削除された値)" {
		t.Errorf("p:cust = %q", got)
	}
	st.Systems[0].Name = "(削除された値)"
	it = item.Item{Kind: item.Task, Systems: []change.ID{st.Systems[0].ID}}
	cells = cellsOf(cellFacts(st, it), columnsOf([]*space{{settings: st}}))
	if slices.Contains(cells.Removed, "system") {
		t.Errorf("一覧にある関連システムが Removed に入っている: %v", cells.Removed)
	}
}

func TestCellsCustomText(t *testing.T) {
	st := defaults()
	single := custom("cust", workspace.TypeSingle)
	single.Choices = []workspace.Value{{ID: "01A", Name: "A社"}}
	multi := custom("tags", workspace.TypeMulti)
	multi.Choices = []workspace.Value{{ID: "01X", Name: "x"}, {ID: "01Y", Name: "y"}}
	user, date, num := custom("owner", workspace.TypeUser), custom("when", workspace.TypeDate), custom("n", workspace.TypeNumber)
	check, memo := custom("ok", workspace.TypeCheck), custom("memo", workspace.TypeText)
	st.Properties = append(st.Properties, single, multi, user, date, num, check, memo)
	it := item.Item{
		Kind:    item.Task,
		Choices: map[change.ID][]change.ID{single.ID: {"01A"}, multi.ID: {"01X", "01Y"}},
		Fields: map[change.ID]json.RawMessage{
			user.ID: json.RawMessage(`"tanaka"`), date.ID: json.RawMessage(`"2026-12-01"`),
			num.ID: json.RawMessage(`3.5`), check.ID: json.RawMessage(`true`),
		},
	}
	f := cellFacts(st, it)
	cols := []Column{
		customCol("cust", workspace.TypeSingle), customCol("tags", workspace.TypeMulti), customCol("owner", workspace.TypeUser),
		customCol("when", workspace.TypeDate), customCol("n", workspace.TypeNumber), customCol("ok", workspace.TypeCheck),
		customCol("memo", workspace.TypeText),
	}
	want := map[string]string{
		"p:cust": "A社", "p:tags": "x, y", "p:owner": "田中", "p:when": "2026-12-01", "p:n": "3.5", "p:ok": "true", "p:memo": "",
	}
	got := cellsOf(f, cols).Custom
	for k, w := range want {
		if got[k].Text != w {
			t.Errorf("%s = %q, want %q", k, got[k].Text, w)
		}
	}
	f.Item.Fields[user.ID] = json.RawMessage(`"nobody"`)
	f.Item.Fields[check.ID] = json.RawMessage(`false`)
	got = cellsOf(f, cols).Custom
	if got["p:owner"].Text != "不明なユーザー" || got["p:ok"].Text != "" {
		t.Errorf("owner = %q, ok = %q", got["p:owner"].Text, got["p:ok"].Text)
	}
}

func TestCellsOwnType(t *testing.T) {
	st := defaults()
	due := custom("due", workspace.TypeText)
	st.Properties = append(st.Properties, due)
	it := item.Item{Kind: item.Task, Fields: map[change.ID]json.RawMessage{due.ID: json.RawMessage(`"来週"`)}}
	got := cellsOf(cellFacts(st, it), []Column{customCol("due", workspace.TypeDate)}).Custom["p:due"]
	if want := (Cell{Text: "来週", Type: workspace.TypeText}); got != want {
		t.Errorf("= %v, want %v", got, want)
	}
}
