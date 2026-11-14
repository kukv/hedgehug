package query

import (
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// customProperty は、key(小文字)と綴りが同じ追加プロパティ。組み込みは含めない。
func customProperty(f Facts, key string) (workspace.Property, bool) {
	for _, p := range f.Settings.Properties {
		if !p.Builtin && strings.EqualFold(p.Key, key) {
			return p, true
		}
	}
	return workspace.Property{}, false
}

// matchCustom は、追加プロパティのキーを型ごとに比べる(04 の追加プロパティの表)。
// キーを持たないワークスペースのアイテムは一致しない。
func matchCustom(t Term, f Facts, today string) bool {
	p, ok := customProperty(f, t.Key)
	if !ok {
		return false
	}
	val := item.FieldValue(f.Item, p)
	switch p.Type {
	case workspace.TypeNumber:
		n, _ := val.(*float64)
		return matchNumber(t, n)
	case workspace.TypeDate:
		return matchDate(t, str(val), today)
	case workspace.TypeUser:
		return anyValue(t, func(v string) bool { return person(v, f, str(val)) })
	case workspace.TypeCheck:
		on, _ := val.(bool)
		return anyValue(t, func(v string) bool { return checked(v, on) })
	case workspace.TypeSingle, workspace.TypeMulti:
		return anyValue(t, func(v string) bool { return chose(p, f.Item.Choices[p.ID], v) })
	case workspace.TypeText, workspace.TypeURL:
		return anyValue(t, func(v string) bool { return contains(str(val), v) })
	}
	return false
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

// checked は、チェックの値 `true`・`false`。false はオフと未設定の両方。
func checked(v string, on bool) bool {
	switch strings.ToLower(v) {
	case "true":
		return on
	case "false":
		return !on
	}
	return false
}

// chose は、選んだ選択肢の名前が v と等しいか。none は何も選んでいない。
func chose(p workspace.Property, ids []change.ID, v string) bool {
	if isNone(v) {
		return len(ids) == 0
	}
	for _, id := range ids {
		if named(p.Choices, id, v) {
			return true
		}
	}
	return false
}

func contains(s, v string) bool {
	if isNone(v) {
		return s == ""
	}
	return strings.Contains(strings.ToLower(s), strings.ToLower(v))
}

// hasField は、追加プロパティ p に値があるか。チェックはオンのときだけ「ある」。
func hasField(f Facts, p workspace.Property) bool {
	switch val := item.FieldValue(f.Item, p).(type) {
	case *float64:
		return val != nil
	case bool:
		return val
	case string:
		return val != ""
	case change.ID:
		return val != ""
	case []change.ID:
		return len(val) > 0
	}
	return false
}
