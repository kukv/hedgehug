package item_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	fNote  change.ID = "01PROP0000000000000000000C" // テキスト
	fCount change.ID = "01PROP0000000000000000000D" // 数値
	fOwner change.ID = "01PROP0000000000000000000E" // ユーザー
	fDone  change.ID = "01PROP0000000000000000000F" // チェック
)

func prop(id change.ID, typ workspace.PropertyType) workspace.Property {
	return workspace.Property{ID: id, Key: string(id), Type: typ}
}

func TestBuiltinValuesRoundTrip(t *testing.T) {
	three, half := 3.0, 1.5
	base := item.Item{
		ID: "01ITEM0000000000000000000A", Kind: item.Task, Title: "x",
		Status: sTodo, Priority: pMedium, Assignee: "taro", Start: "2026-11-11", End: "2026-11-15",
		Points: &three, Estimate: &half, Systems: []change.ID{yWeb},
	}
	got, ok := item.FromLog(rows(item.Create(base)))
	if !ok || got.Assignee != "taro" || got.Start != "2026-11-11" || got.End != "2026-11-15" ||
		*got.Points != 3 || *got.Estimate != 1.5 || got.Status != sTodo || got.Priority != pMedium {
		t.Errorf("FromLog = %+v", got)
	}
}

func TestEmptyValuesAreNotWritten(t *testing.T) {
	d := item.Create(item.Item{ID: "01ITEM0000000000000000000A", Kind: item.Task, Title: "x"})
	var to map[string]json.RawMessage
	_ = json.Unmarshal(d.To, &to)
	for _, k := range []string{"assignee", "start", "end", "points", "estimate", "systems"} {
		if _, ok := to[k]; ok {
			t.Errorf("create wrote %s", k)
		}
	}
}

// 内部 ID の形をしたテキストとログイン名も、選択肢ではなくテキスト・ユーザーとして読める(決定 9)。
func TestFieldValueReadsByTheSettingsType(t *testing.T) {
	idShaped := json.RawMessage(`"01CHOICE00000000000000000A"`)
	base := item.Item{
		ID: "01ITEM0000000000000000000A", Kind: item.Task, Title: "x",
		Fields: map[change.ID]json.RawMessage{
			fNote: idShaped, fCount: json.RawMessage(`2.5`), fOwner: json.RawMessage(`"hanako"`),
			fDone: json.RawMessage(`true`), fEnv: json.RawMessage(`"` + string(vProd) + `"`),
			fTags: json.RawMessage(`["` + string(vA) + `","` + string(vB) + `"]`),
		},
	}
	it, _ := item.FromLog(rows(item.Create(base)))
	cases := []struct {
		p    workspace.Property
		want any
	}{
		{prop(fNote, workspace.TypeText), "01CHOICE00000000000000000A"},
		{prop(fCount, workspace.TypeNumber), ptr(2.5)},
		{prop(fOwner, workspace.TypeUser), "hanako"},
		{prop(fDone, workspace.TypeCheck), true},
		{prop(fEnv, workspace.TypeSingle), vProd},
		{prop(fTags, workspace.TypeMulti), []change.ID{vA, vB}},
		{prop("01PROP0000000000000000000Z", workspace.TypeCheck), false},
	}
	for _, c := range cases {
		if got := item.FieldValue(it, c.p); !reflect.DeepEqual(got, c.want) {
			t.Errorf("FieldValue(%s) = %#v, want %#v", c.p.Type, got, c.want)
		}
	}
}

func TestCurrentIsTheLogForm(t *testing.T) {
	half := 1.5
	it := item.Item{
		Assignee: "taro", Estimate: &half, Systems: []change.ID{yWeb},
		Fields: map[change.ID]json.RawMessage{fNote: json.RawMessage(`"memo"`)},
	}
	cases := map[string]struct {
		p    workspace.Property
		want string
	}{
		"assignee": {workspace.Property{ID: "assignee", Key: "assignee", Builtin: true, Type: workspace.TypeUser}, `"taro"`},
		"start":    {workspace.Property{ID: "start", Key: "start", Builtin: true, Type: workspace.TypeDate}, `null`},
		"estimate": {workspace.Property{ID: "estimate", Key: "estimate", Builtin: true, Type: workspace.TypeDuration}, `1.5`},
		"systems":  {workspace.Property{ID: "systems", Key: "systems", Builtin: true, Type: workspace.TypeSystems}, `["` + string(yWeb) + `"]`},
		"text":     {prop(fNote, workspace.TypeText), `"memo"`},
		"check":    {prop(fDone, workspace.TypeCheck), `false`},
	}
	for name, c := range cases {
		if got := string(item.Current(it, c.p)); got != c.want {
			t.Errorf("%s: Current = %s, want %s", name, got, c.want)
		}
		if name == "text" && item.Field(c.p) != "fields/"+string(fNote) {
			t.Errorf("Field = %s", item.Field(c.p))
		}
	}
}

func ptr(f float64) *float64 { return &f }
