package query

import (
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// matchNames は、名前・ログイン名・ID のキーを判定する。2 つ目は、このキーを扱ったか。
// 比べ方は大文字と小文字を区別しない完全一致(E1-5)。`:` 以外の演算子は `:` として読む(04 の 233 行)。
func matchNames(t Term, f Facts) (bool, bool) {
	if idKey(t.Key) {
		return anyValue(t, func(v string) bool { return matchID(t.Key, v, f) }), true
	}
	eq := nameMatcher(t.Key, f)
	if eq == nil {
		return false, false
	}
	return anyValue(t, eq), true
}

// nameMatcher は、名前のキーの 1 つの値の判定を返す。名前のキーでなければ nil。
func nameMatcher(key string, f Facts) func(v string) bool {
	switch key {
	case "status":
		return func(v string) bool { return named(f.Settings.Statuses, f.Item.Status, v) }
	case "priority":
		return func(v string) bool { return named(f.Settings.Priorities, f.Item.Priority, v) }
	case "category":
		return func(v string) bool { return category(f, v) }
	case "kind":
		return func(v string) bool { return strings.EqualFold(string(f.Item.Kind), v) }
	case "system":
		return func(v string) bool { return system(f, v) }
	case "assignee":
		return func(v string) bool { return person(v, f, f.Item.Assignee) }
	case "watching":
		return func(v string) bool { return person(v, f, f.Item.Watchers...) }
	case "workspace":
		return func(v string) bool {
			return strings.EqualFold(f.Workspace.Name, v) || strings.EqualFold(f.Workspace.Prefix, v)
		}
	case "store":
		return func(v string) bool { return strings.EqualFold(f.StoreName, v) }
	}
	return nil
}

func anyValue(t Term, eq func(string) bool) bool {
	for _, v := range t.Values {
		if eq(v) {
			return true
		}
	}
	return false
}

func isNone(v string) bool { return strings.EqualFold(v, "none") }

func idKey(key string) bool {
	switch key {
	case "project", "parent", "epic":
		return true
	}
	return relationKeys[key]
}

var relationKeys = map[string]bool{
	"blocks": true, "blocked_by": true, "causes": true, "caused_by": true, "duplicates": true,
	"duplicated_by": true, "relates_to": true, "clones": true, "cloned_by": true,
}

// named は、値 ID が指す名前が v と等しいか。`none` は値なし。
func named(values []workspace.Value, id change.ID, v string) bool {
	if isNone(v) {
		return id == ""
	}
	for _, x := range values {
		if x.ID == id {
			return strings.EqualFold(x.Name, v)
		}
	}
	return false
}

func category(f Facts, v string) bool {
	for _, x := range f.Settings.Statuses {
		if x.ID == f.Item.Status {
			return strings.EqualFold(string(x.Category), v)
		}
	}
	return false
}

func system(f Facts, v string) bool {
	if isNone(v) {
		return len(f.Item.Systems) == 0
	}
	for _, id := range f.Item.Systems {
		if named(f.Settings.Systems, id, v) {
			return true
		}
	}
	return false
}

// person は、ログイン名の値 v が logins のどれかと等しいか。先頭の `@` は除く。`@me` は Facts.Me(空なら一致しない)。
func person(v string, f Facts, logins ...string) bool {
	v = strings.TrimPrefix(v, "@")
	if isNone(v) {
		for _, l := range logins {
			if l != "" {
				return false
			}
		}
		return true
	}
	if strings.EqualFold(v, "me") {
		if f.Me == "" {
			return false
		}
		v = f.Me
	}
	for _, l := range logins {
		if l != "" && strings.EqualFold(l, v) {
			return true
		}
	}
	return false
}

func refIs(r Ref, v string) bool {
	if strings.EqualFold(r.Display, v) {
		return true
	}
	for _, a := range r.Aliases {
		if strings.EqualFold(a, v) {
			return true
		}
	}
	return false
}

func anyRef(rs []Ref, v string) bool {
	for _, r := range rs {
		if refIs(r, v) {
			return true
		}
	}
	return false
}

// matchID は、ID のキー(project・parent・epic・関連)の 1 つの値を判定する(E1-6)。
func matchID(key, v string, f Facts) bool {
	switch key {
	case "project":
		return !isNone(v) && refIs(f.Project, v)
	case "parent", "epic":
		if isNone(v) {
			return f.Parent == nil
		}
		return (f.Parent != nil && refIs(*f.Parent, v)) || anyRef(f.Ancestors, v)
	}
	if isNone(v) {
		return len(f.Relations[key]) == 0
	}
	return anyRef(f.Relations[key], v)
}

// freeWord は、自由な語がいまの ID・旧 ID・タイトル・説明のどれかに部分一致するか。
// 2 つ目は、旧 ID だけで一致したこと。
func freeWord(w string, f Facts) (bool, bool) {
	w = strings.ToLower(w)
	has := func(s string) bool { return strings.Contains(strings.ToLower(s), w) }
	if has(f.Display) || has(f.Item.Title) || has(f.Description) {
		return true, false
	}
	for _, a := range f.Aliases {
		if has(a) {
			return true, true
		}
	}
	return false, false
}
