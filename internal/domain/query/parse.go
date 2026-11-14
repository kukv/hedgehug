package query

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Op は、キーと値のあいだの演算子。
type Op string

const (
	OpEq Op = ":"
	OpLt Op = "<"
	OpLe Op = "<="
	OpGt Op = ">"
	OpGe Op = ">="
)

// Term は、クエリの 1 つの語。
type Term struct {
	Raw    string   // 書いたとおりの語(`-` と引用符を含む)
	Not    bool     // 先頭の `-`(`-` だけの語は否定にしない)
	Key    string   // 小文字。自由な語は ""
	Op     Op       // キーのある語の演算子
	Values []string // キーのある語は `,` で区切った値。自由な語は語 1 つ
}

// Sort は、`sort:` の指定。
type Sort struct {
	Key  string
	Desc bool
}

// Query は、解析したクエリ。
type Query struct {
	Terms        []Term // sort・group を除く
	Sort         *Sort
	Group        string
	Deleted      bool // 否定でない `is:deleted` がある
	HasKey       bool // キーのある語がある(sort・group を含む)
	NeedsText    bool // 自由な語か `mentions:` がある(説明とコメントを読む)
	NeedsInbox   bool // `is:overwritten` がある
	NeedsInvalid bool // `is:invalid` がある
}

var keyPattern = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_]*)(:|<=|>=|<|>)`)

// Parse は、text を語に切って解析する。2 つ目は、エラーの文(書いた順)。custom は、小文字のキーが
// どれかのワークスペースの追加プロパティにあるか。
func Parse(text string, custom func(key string) bool) (Query, []string) {
	var q Query
	var errs []string
	for _, raw := range split(text) {
		t := term(raw)
		if t.Key == "" && strings.TrimSpace(t.Values[0]) == "" {
			continue // 空の自由な語(`""`・`" "`)は条件にしない
		}
		if t.Key != "" {
			q.HasKey = true
		}
		if msg := check(t, custom); msg != "" {
			errs = append(errs, msg)
		}
		q.add(t)
	}
	return q, errs
}

// split は、空白で語に切る。引用符のあいだの空白は区切らず、閉じていない引用符は末尾までを 1 つの語にする。
func split(text string) []string {
	var out []string
	var cur strings.Builder
	inQuote := false
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, r := range text {
		switch {
		case r == '"':
			inQuote = !inQuote
			cur.WriteRune(r)
		case unicode.IsSpace(r) && !inQuote:
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return out
}

// term は、1 つの語を Term にする。キーは引用符を除く前の形で見分ける(`"status:x"` は自由な語)。
func term(raw string) Term {
	t := Term{Raw: raw}
	body := raw
	if len(raw) > 1 && raw[0] == '-' {
		t.Not = true
		body = raw[1:]
	}
	m := keyPattern.FindStringSubmatch(body)
	if m == nil {
		t.Values = []string{strings.ReplaceAll(body, `"`, "")}
		return t
	}
	t.Key = strings.ToLower(m[1])
	t.Op = Op(m[2])
	rest := strings.ReplaceAll(body[len(m[0]):], `"`, "")
	for _, v := range strings.Split(rest, ",") {
		if v = strings.TrimSpace(v); v != "" {
			t.Values = append(t.Values, v)
		}
	}
	return t
}

// check は、語のエラーの文を返す。なければ ""。最初の 1 つだけ(E1-3)。
func check(t Term, custom func(key string) bool) string {
	if t.Key == "" {
		return ""
	}
	if !Builtin(t.Key) && (custom == nil || !custom(t.Key)) {
		return fmt.Sprintf("「%s」というキーはありません", t.Key)
	}
	if len(t.Values) == 0 {
		return fmt.Sprintf("「%s」に値がありません", t.Raw)
	}
	if noneForbidden[t.Key] && hasValue(t, "none") {
		return fmt.Sprintf("%s に none は使えません", t.Key)
	}
	return ""
}

func hasValue(t Term, want string) bool {
	for _, v := range t.Values {
		if strings.EqualFold(v, want) {
			return true
		}
	}
	return false
}

// add は、語を結果に入れる。sort・group は Terms に入れず、最初のものだけを使う。
func (q *Query) add(t Term) {
	switch t.Key {
	case "":
		q.NeedsText = true
	case "sort":
		if q.Sort == nil && len(t.Values) > 0 {
			v := t.Values[0]
			q.Sort = &Sort{Key: strings.ToLower(strings.TrimPrefix(v, "-")), Desc: strings.HasPrefix(v, "-")}
		}
		return
	case "group":
		if q.Group == "" && len(t.Values) > 0 {
			q.Group = strings.ToLower(strings.TrimPrefix(t.Values[0], "-"))
		}
		return
	case "mentions":
		q.NeedsText = true
	case "is":
		q.Deleted = q.Deleted || (!t.Not && hasValue(t, "deleted"))
		q.NeedsInbox = q.NeedsInbox || hasValue(t, "overwritten")
		q.NeedsInvalid = q.NeedsInvalid || hasValue(t, "invalid")
	}
	q.Terms = append(q.Terms, t)
}
