package schema

import (
	"slices"
	"strings"
)

// Builtin is one of the six built-in properties of a task. Each is a field of
// the task with the same name, outside props.
type Builtin struct {
	Name, Label string
	Type        PropType
}

// Builtins are the built-in properties, in the order shown. priority and
// labels take their values from the lists of the schema (Schema.Priorities
// and Schema.Labels); the others have none.
var Builtins = []Builtin{
	{"priority", "優先度", TypeEnum},
	{"start", "開始日", TypeDate},
	{"due", "期限", TypeDate},
	{"estimate", "見積もり", TypeDuration},
	{"points", "ポイント", TypeNumber},
	{"labels", "ラベル", TypeMultiEnum},
}

// BuiltinOf finds a built-in property by its name.
func BuiltinOf(name string) (Builtin, bool) {
	i := slices.IndexFunc(Builtins, func(b Builtin) bool { return b.Name == name })
	if i < 0 {
		return Builtin{}, false
	}
	return Builtins[i], true
}

// MatchValue finds v among values without regard to case and returns the
// spelling in values, which is the one that is stored. An exact match wins
// over one that differs in case.
func MatchValue(values []string, v string) (string, bool) {
	if slices.Contains(values, v) {
		return v, true
	}
	i := slices.IndexFunc(values, func(x string) bool { return strings.EqualFold(x, v) })
	if i < 0 {
		return "", false
	}
	return values[i], true
}
