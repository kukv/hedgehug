package query

// builtinKeys は、組み込みのキー(小文字)。追加プロパティが同じ綴りでも、組み込みの意味を使う。
var builtinKeys = map[string]bool{
	"status": true, "category": true, "kind": true, "system": true, "assignee": true, "watching": true,
	"mentions": true, "priority": true, "sprint": true, "release": true, "project": true, "parent": true,
	"epic": true, "blocks": true, "blocked_by": true, "causes": true, "caused_by": true, "duplicates": true,
	"duplicated_by": true, "relates_to": true, "clones": true, "cloned_by": true, "workspace": true,
	"store": true, "is": true, "has": true, "points": true, "estimate": true, "start": true, "end": true,
	"created": true, "updated": true, "sort": true, "group": true,
}

// noneForbidden は、`none` を書けないキー。
var noneForbidden = map[string]bool{"priority": true, "created": true, "updated": true}

// Builtin は、key(小文字)が組み込みのキーか。
func Builtin(key string) bool {
	return builtinKeys[key]
}
