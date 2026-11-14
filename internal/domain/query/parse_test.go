package query_test

import (
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/query"
)

type want struct {
	key    string
	not    bool
	op     query.Op
	values []string
}

func TestParse(t *testing.T) {
	cases := []struct {
		in    string
		terms []want
		errs  []string
	}{
		{"status:InProgress kind:story,bug", []want{{"status", false, ":", []string{"InProgress"}}, {"kind", false, ":", []string{"story", "bug"}}}, nil},
		{"Status:done", []want{{"status", false, ":", []string{"done"}}}, nil},
		{"-assignee:@me", []want{{"assignee", true, ":", []string{"@me"}}}, nil},
		{`release:"v2.0 beta"`, []want{{"release", false, ":", []string{"v2.0 beta"}}}, nil},
		{`release:"v2.0 beta`, []want{{"release", false, ":", []string{"v2.0 beta"}}}, nil},
		{`"ガント 表"`, []want{{"", false, "", []string{"ガント 表"}}}, nil},
		{`"" " " foo`, []want{{"", false, "", []string{"foo"}}}, nil},
		{"- foo", []want{{"", false, "", []string{"-"}}, {"", false, "", []string{"foo"}}}, nil},
		{"points>=3 end<today", []want{{"points", false, ">=", []string{"3"}}, {"end", false, "<", []string{"today"}}}, nil},
		{"http://x", []want{{"http", false, ":", []string{"//x"}}}, []string{"「http」というキーはありません"}},
		{"v2<v3", []want{{"v2", false, "<", []string{"v3"}}}, []string{"「v2」というキーはありません"}},
		{"C++>11", []want{{"", false, "", []string{"C++>11"}}}, nil},
		{"status:", []want{{"status", false, ":", nil}}, []string{"「status:」に値がありません"}},
		{"status:,", []want{{"status", false, ":", nil}}, []string{"「status:,」に値がありません"}},
		{"-status:", []want{{"status", true, ":", nil}}, []string{"「-status:」に値がありません"}},
		{"priority:none", []want{{"priority", false, ":", []string{"none"}}}, []string{"priority に none は使えません"}},
		{"created:none", []want{{"created", false, ":", []string{"none"}}}, []string{"created に none は使えません"}},
		{"-updated:none", []want{{"updated", true, ":", []string{"none"}}}, []string{"updated に none は使えません"}},
		{
			"foo:x status:",
			[]want{{"foo", false, ":", []string{"x"}}, {"status", false, ":", nil}},
			[]string{"「foo」というキーはありません", "「status:」に値がありません"},
		},
		{"", nil, nil},
	}
	for _, c := range cases {
		q, errs := query.Parse(c.in, nil)
		var got []want
		for _, tm := range q.Terms {
			got = append(got, want{tm.Key, tm.Not, tm.Op, tm.Values})
		}
		if !reflect.DeepEqual(got, c.terms) {
			t.Errorf("%q: terms = %+v, want %+v", c.in, got, c.terms)
		}
		if !reflect.DeepEqual(errs, c.errs) {
			t.Errorf("%q: errs = %v, want %v", c.in, errs, c.errs)
		}
	}
}

func TestParseCustomKey(t *testing.T) {
	custom := func(k string) bool { return k == "customer" }
	if _, errs := query.Parse("customer:x", custom); len(errs) != 0 {
		t.Errorf("customer:x errs = %v", errs)
	}
	_, errs := query.Parse("Customer:x", func(string) bool { return false })
	if !reflect.DeepEqual(errs, []string{"「customer」というキーはありません"}) {
		t.Errorf("Customer:x errs = %v", errs)
	}
}

func TestParseSortGroup(t *testing.T) {
	cases := []struct {
		in    string
		sort  *query.Sort
		group string
		terms int
	}{
		{"-sort:priority", &query.Sort{Key: "priority"}, "", 0},
		{"sort:-priority sort:id", &query.Sort{Key: "priority", Desc: true}, "", 0},
		{"group:kind,status -group:-x", nil, "kind", 0},
		{"group:-kind", nil, "kind", 0},
		{"kind:bug", nil, "", 1},
	}
	for _, c := range cases {
		q, errs := query.Parse(c.in, nil)
		if len(errs) != 0 {
			t.Errorf("%q: errs = %v", c.in, errs)
		}
		if !reflect.DeepEqual(q.Sort, c.sort) || q.Group != c.group || len(q.Terms) != c.terms {
			t.Errorf("%q: sort=%+v group=%q terms=%d", c.in, q.Sort, q.Group, len(q.Terms))
		}
	}
}

func TestParseFlags(t *testing.T) {
	cases := []struct {
		in                                    string
		deleted, hasKey, text, inbox, invalid bool
	}{
		{"is:deleted,blocked", true, true, false, false, false},
		{"-is:deleted", false, true, false, false, false},
		{"foo", false, false, true, false, false},
		{"foo sort:id", false, true, true, false, false},
		{"mentions:a", false, true, true, false, false},
		{"kind:bug", false, true, false, false, false},
		{"is:overwritten", false, true, false, true, false},
		{"-is:invalid", false, true, false, false, true},
		{"", false, false, false, false, false},
		{`"" " "`, false, false, false, false, false},
	}
	for _, c := range cases {
		q, _ := query.Parse(c.in, nil)
		got := []bool{q.Deleted, q.HasKey, q.NeedsText, q.NeedsInbox, q.NeedsInvalid}
		exp := []bool{c.deleted, c.hasKey, c.text, c.inbox, c.invalid}
		if !reflect.DeepEqual(got, exp) {
			t.Errorf("%q: flags = %v, want %v", c.in, got, exp)
		}
	}
}

func TestBuiltin(t *testing.T) {
	if !query.Builtin("blocked_by") || query.Builtin("customer") {
		t.Error("Builtin")
	}
}
