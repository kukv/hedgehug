package workspace

import (
	"regexp"
	"slices"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// ReservedKeys は、検索の条件のキー(03「追加プロパティ」の 34 個)。
var ReservedKeys = []string{
	"status", "kind", "system", "assignee", "priority", "sprint", "release", "project", "parent", "epic",
	"workspace", "store", "is", "has", "watching", "sort", "points", "estimate", "start", "end",
	"category", "group", "mentions", "created", "updated", "blocks", "blocked_by", "causes", "caused_by",
	"duplicates", "duplicated_by", "relates_to", "clones", "cloned_by",
}

// AddableTypes は、追加プロパティに選べる 8 型(03「追加プロパティ」の表の順)。
var AddableTypes = []PropertyType{TypeText, TypeNumber, TypeDate, TypeSingle, TypeMulti, TypeUser, TypeCheck, TypeURL}

var keyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// NormalizeKinds は、使う種類を Kinds の並びにし、重複と知らない種類を除く。
func NormalizeKinds(kinds []string) []string {
	var out []string
	for _, k := range Kinds {
		if slices.Contains(kinds, k) {
			out = append(out, k)
		}
	}
	return out
}

// propertyNameTaken は、id 以外に同じ名前のプロパティがあるか(組み込みと非表示を含む。完全一致)。
func propertyNameTaken(s Settings, id change.ID, name string) bool {
	return slices.ContainsFunc(s.Properties, func(p Property) bool { return p.ID != id && p.Name == name })
}

// CheckNewProperty は、プロパティの追加の検査(03「追加プロパティ」の表の順)。name は NormalizeName を通した形。
func CheckNewProperty(s Settings, name, key string) error {
	switch {
	case name == "":
		return refusal.New("名前を入力してください")
	case propertyNameTaken(s, "", name):
		return refusal.New("同じ名前のプロパティがあります")
	case !keyPattern.MatchString(key):
		return refusal.New("キーは英小文字で始まる英小文字・数字・_ にしてください")
	case slices.Contains(ReservedKeys, key) ||
		slices.ContainsFunc(s.Properties, func(p Property) bool { return p.Key == key }):
		return refusal.New("このキーは使用できません(もうあるか、予約されています)")
	}
	return nil
}

// CheckPropertyName は、プロパティの改名の検査(03「一覧」の名前)。
func CheckPropertyName(s Settings, id change.ID, name string) error {
	switch {
	case name == "":
		return refusal.New("名前は空にできません")
	case propertyNameTaken(s, id, name):
		return refusal.New("同じ名前のプロパティがあります")
	}
	return nil
}

// CheckKinds は、使う種類の変更の検査(03「使う種類」)。kinds は NormalizeKinds を通した形。
func CheckKinds(kinds []string) error {
	if len(kinds) == 0 {
		return refusal.New("少なくとも 1 つの種類で使ってください")
	}
	return nil
}
