package query

import "strings"

// UsesKey は、組み込みでないキー key を使う語が、否定を含めて 1 つでもあるか。大文字と小文字を区別しない。
func UsesKey(text, key string) bool {
	key = strings.ToLower(key)
	if Builtin(key) {
		return false
	}
	q, _ := Parse(text, func(string) bool { return true })
	for _, t := range q.Terms {
		if t.Key == key {
			return true
		}
	}
	return false
}
