// Package query parses and runs the query language that hh ls and the
// GUI's filter share, over the index of a store.
package query

import (
	"fmt"
	"strings"
	"unicode"
)

// Query is a parsed query: terms that must all match.
type Query struct{ Terms []Term }

// Term is one space-separated part of a query.
type Term struct {
	Pos    int      // 1-based position in the query
	Raw    string   // the term as written
	Neg    bool     // led by -
	Key    string   // empty for free text
	Op     string   // ":", "<", ">", "<=" or ">="; empty for free text
	Values []string // any of them matches; one for free text
}

// Error is a query that cannot be parsed or run. Pos is the term it is
// about.
type Error struct {
	Pos  int
	Term string
	Msg  string
	Err  error // the error Msg tells of, when there is one
	// Unknown says the term names a property the schema does not have: an
	// unknown key, a sort: or group: field that is not there, or a
	// comparison on a property that is not there. mount.Query treats a store
	// with this error as having no match.
	Unknown bool
}

func (e *Error) Error() string { return fmt.Sprintf("term %d (%s): %s", e.Pos, e.Term, e.Msg) }

func (e *Error) Unwrap() error { return e.Err }

// Parse parses s. It checks the syntax only; Run checks keys and values
// against the store's schema.
func Parse(s string) (*Query, error) {
	toks, err := split(s)
	if err != nil {
		return nil, err
	}
	q := &Query{}
	for i, tok := range toks {
		t, err := parseTerm(i+1, tok)
		if err != nil {
			return nil, err
		}
		q.Terms = append(q.Terms, t)
	}
	return q, nil
}

// split cuts s at white space outside double quotes. The quotes stay in
// the tokens.
func split(s string) ([]string, error) {
	var out []string
	var b strings.Builder
	quoted := false
	for _, r := range s {
		switch {
		case r == '"':
			quoted = !quoted
			b.WriteRune(r)
		case unicode.IsSpace(r) && !quoted:
			if b.Len() > 0 {
				out = append(out, b.String())
				b.Reset()
			}
		default:
			b.WriteRune(r)
		}
	}
	if quoted {
		return nil, &Error{Pos: len(out) + 1, Term: b.String(), Msg: "unclosed quote"}
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out, nil
}

func parseTerm(pos int, tok string) (Term, error) {
	fail := func(msg string) (Term, error) { return Term{}, &Error{Pos: pos, Term: tok, Msg: msg} }
	t := Term{Pos: pos, Raw: tok}
	body := tok
	if strings.HasPrefix(body, "-") {
		t.Neg = true
		body = body[1:]
	}
	if body == "" {
		return fail("empty term")
	}
	i, op := operator(body)
	if i < 0 {
		v := unquote(body)
		if v == "" {
			return fail("empty text")
		}
		t.Values = []string{v}
		return t, nil
	}
	t.Key, t.Op = body[:i], op
	if t.Key == "" {
		return fail("missing key")
	}
	if strings.Contains(t.Key, `"`) {
		return fail("a key cannot be quoted")
	}
	for _, v := range splitValues(body[i+len(op):]) {
		v = unquote(v)
		if v == "" {
			return fail("empty value")
		}
		t.Values = append(t.Values, v)
	}
	return t, nil
}

// operator finds the first ':', "<=", ">=", '<' or '>' outside quotes, and
// returns where it is and which it is; -1 when there is none.
func operator(s string) (int, string) {
	quoted := false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '"':
			quoted = !quoted
		case quoted:
		case c == ':':
			return i, ":"
		case c == '<' || c == '>':
			if i+1 < len(s) && s[i+1] == '=' {
				return i, s[i : i+2]
			}
			return i, s[i : i+1]
		}
	}
	return -1, ""
}

// splitValues cuts s at commas outside quotes.
func splitValues(s string) []string {
	var out []string
	quoted, start := false, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			quoted = !quoted
		case ',':
			if !quoted {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	return append(out, s[start:])
}

func unquote(s string) string { return strings.ReplaceAll(s, `"`, "") }
