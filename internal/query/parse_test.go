package query

import (
	"errors"
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want []Term
	}{
		{"", nil},
		{"status:doing", []Term{{Pos: 1, Raw: "status:doing", Key: "status", Op: ":", Values: []string{"doing"}}}},
		{"-label:wip", []Term{{Pos: 1, Raw: "-label:wip", Neg: true, Key: "label", Op: ":", Values: []string{"wip"}}}},
		{"priority:high,urgent", []Term{{Pos: 1, Raw: "priority:high,urgent", Key: "priority", Op: ":", Values: []string{"high", "urgent"}}}},
		{"due<2026-11-10", []Term{{Pos: 1, Raw: "due<2026-11-10", Key: "due", Op: "<", Values: []string{"2026-11-10"}}}},
		{"points>=3", []Term{{Pos: 1, Raw: "points>=3", Key: "points", Op: ">=", Values: []string{"3"}}}},
		{"points<=3", []Term{{Pos: 1, Raw: "points<=3", Key: "points", Op: "<=", Values: []string{"3"}}}},
		{"points>3", []Term{{Pos: 1, Raw: "points>3", Key: "points", Op: ">", Values: []string{"3"}}}},
		{`"検索語"`, []Term{{Pos: 1, Raw: `"検索語"`, Values: []string{"検索語"}}}},
		{"login", []Term{{Pos: 1, Raw: "login", Values: []string{"login"}}}},
		{`-"a b"`, []Term{{Pos: 1, Raw: `-"a b"`, Neg: true, Values: []string{"a b"}}}},
		{`"a:b"`, []Term{{Pos: 1, Raw: `"a:b"`, Values: []string{"a:b"}}}},
		{`label:"needs review"`, []Term{{Pos: 1, Raw: `label:"needs review"`, Key: "label", Op: ":", Values: []string{"needs review"}}}},
		{`label:"a,b",c`, []Term{{Pos: 1, Raw: `label:"a,b",c`, Key: "label", Op: ":", Values: []string{"a,b", "c"}}}},
		{"  kind:task \t is:blocked ", []Term{
			{Pos: 1, Raw: "kind:task", Key: "kind", Op: ":", Values: []string{"task"}},
			{Pos: 2, Raw: "is:blocked", Key: "is", Op: ":", Values: []string{"blocked"}},
		}},
	}
	for _, tt := range tests {
		q, err := Parse(tt.in)
		if err != nil {
			t.Errorf("Parse(%q): %v", tt.in, err)
			continue
		}
		if !reflect.DeepEqual(q.Terms, tt.want) {
			t.Errorf("Parse(%q) = %+v, want %+v", tt.in, q.Terms, tt.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in  string
		pos int
	}{
		{"status:", 1},
		{":x", 1},
		{"-", 1},
		{`"abc`, 1},
		{`kind:task "abc`, 2},
		{"kind:task label:a,,b", 2},
		{"label:a,", 1},
		{`-""`, 1},
	}
	for _, tt := range tests {
		_, err := Parse(tt.in)
		var qe *Error
		if !errors.As(err, &qe) || qe.Pos != tt.pos {
			t.Errorf("Parse(%q) err = %v, want an Error at term %d", tt.in, err, tt.pos)
		}
	}
}

func FuzzParse(f *testing.F) {
	for _, s := range []string{"status:doing", `-label:"a,b",c`, `"x`, "due<=2026-11-10", "-", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = Parse(s) // must not panic
	})
}
