package item

import (
	"encoding/json"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Field は、プロパティ p の値を持つログの項目名(決定 7)。組み込みはキー、追加は fields/<ID>。
func Field(p workspace.Property) string {
	if p.Builtin {
		return p.Key
	}
	return FieldKey(p.ID)
}

// Current は、p のいまの値を、ログに書く形(JSON)で返す。値がなければ null(チェックは false)。
// 関連システムと複数選択は、値の内部 ID の配列。
func Current(it Item, p workspace.Property) json.RawMessage {
	if p.Builtin {
		return builtinValue(it, p.Key)
	}
	switch p.Type {
	case workspace.TypeMulti:
		return change.Value(nonNil(it.Choices[p.ID]))
	case workspace.TypeCheck:
		return change.Value(FieldValue(it, p))
	}
	if raw, ok := it.Fields[p.ID]; ok {
		return raw
	}
	return json.RawMessage("null")
}

func builtinValue(it Item, key string) json.RawMessage {
	opt := func(s string) any {
		if s == "" {
			return nil
		}
		return s
	}
	switch key {
	case "status":
		return change.Value(opt(string(it.Status)))
	case "priority":
		return change.Value(opt(string(it.Priority)))
	case "assignee":
		return change.Value(opt(it.Assignee))
	case "start":
		return change.Value(opt(it.Start))
	case "end":
		return change.Value(opt(it.End))
	case "points":
		return change.Value(it.Points)
	case "estimate":
		return change.Value(it.Estimate)
	case "systems":
		return change.Value(nonNil(it.Systems))
	}
	return json.RawMessage("null")
}

// FieldValue は、追加プロパティ p の値を p の型で読む(決定 9)。テキスト・URL・日付・ユーザーは string、
// 数値は *float64、チェックは bool、単一選択は change.ID、複数選択は []change.ID。値がなければゼロ値。
func FieldValue(it Item, p workspace.Property) any {
	raw := it.Fields[p.ID]
	switch p.Type {
	case workspace.TypeSingle:
		if vs := it.Choices[p.ID]; len(vs) > 0 {
			return vs[0]
		}
		return change.ID("")
	case workspace.TypeMulti:
		return it.Choices[p.ID]
	case workspace.TypeNumber:
		return num(raw)
	case workspace.TypeCheck:
		var b bool
		_ = json.Unmarshal(raw, &b)
		return b
	}
	return str(raw)
}

func nonNil(ids []change.ID) []change.ID {
	if ids == nil {
		return []change.ID{}
	}
	return ids
}
