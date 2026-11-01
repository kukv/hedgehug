package query

import (
	"errors"
	"testing"
)

func TestCompareSortKeys(t *testing.T) {
	none := SortKey{None: true}
	place := func(class, place int, text string) SortKey {
		return SortKey{Class: class, Place: place, Text: text, By: "place"}
	}
	num := func(x float64, ok bool, text string) SortKey {
		return SortKey{Num: x, NumOK: ok, Text: text, By: "number"}
	}
	text := func(s string) SortKey { return SortKey{Text: s, By: "text"} }
	tests := []struct {
		name string
		a, b SortKey
		desc bool
		want int
	}{
		{"none is last", none, text("a"), false, 1},
		{"none is last the other way", text("a"), none, false, -1},
		{"none is last when descending", none, text("a"), true, 1},
		{"none is last the other way when descending", text("a"), none, true, -1},
		{"two nones", none, none, true, 0},
		{"class before place", place(0, 5, "x"), place(1, 0, "y"), false, -1},
		{"class reversed by desc", place(0, 5, "x"), place(1, 0, "y"), true, 1},
		{"place", place(1, 1, "b"), place(1, 2, "a"), false, -1},
		{"place ties by text", place(1, 3, "b"), place(1, 3, "a"), false, 1},
		{"place is equal", place(1, 3, "a"), place(1, 3, "a"), false, 0},
		{"place reversed by desc", place(1, 1, "b"), place(1, 2, "a"), true, 1},
		{"number", num(1, true, "1"), num(2, true, "2"), false, -1},
		{"number reversed by desc", num(1, true, "1"), num(2, true, "2"), true, 1},
		{"parsed before not", num(9, true, "9"), num(0, false, "a"), false, -1},
		{"not parsed after, desc", num(9, true, "9"), num(0, false, "a"), true, 1},
		{"equal numbers by text", num(1, true, "1.0"), num(1, true, "1"), false, 1},
		{"neither parses, by text", num(0, false, "b"), num(0, false, "a"), false, 1},
		{"text ignores case", text("Apple"), text("banana"), false, -1},
		{"text equal ignoring case", text("A"), text("a"), false, 0},
		{"text desc", text("a"), text("b"), true, 1},
	}
	for _, tt := range tests {
		if got := CompareSortKeys(tt.a, tt.b, tt.desc); got != tt.want {
			t.Errorf("%s: = %d; want %d", tt.name, got, tt.want)
		}
	}
}

func TestRunGivesSortKeys(t *testing.T) {
	ix := sortFixture(t)
	parsed := func(s string) *Query {
		q, err := Parse(s)
		if err != nil {
			t.Fatal(err)
		}
		return q
	}
	res, err := Run(ix, parsed("group:status"), Options{})
	if err != nil || res.SortKey != nil {
		t.Fatalf("without sort: SortKey set = %v, %v; want nil", res.SortKey != nil, err)
	}
	res, err = Run(ix, parsed("sort:-status"), Options{})
	if err != nil || res.SortKey == nil || !res.Desc {
		t.Fatalf("sort:-status: key %v, desc %v, %v", res.SortKey != nil, res.Desc, err)
	}
	by := map[string]SortKey{}
	for _, e := range res.Groups[0].Tasks {
		by[e.Title] = res.SortKey(e)
	}
	if k := by["t1"]; k.Class != 0 || k.By != "place" || k.Text != "todo" {
		t.Errorf("t1 = %+v; want the todo category", k)
	}
	if k := by["t2"]; k.Class != 1 {
		t.Errorf("t2 = %+v; want the doing category", k)
	}
	if k := by["t3"]; k.Class != 2 {
		t.Errorf("t3 = %+v; want the done category", k)
	}
	res, err = Run(ix, parsed("sort:due"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range res.Groups[0].Tasks {
		k := res.SortKey(e)
		switch e.Title {
		case "t2":
			if !k.None {
				t.Errorf("t2 has no due, key = %+v", k)
			}
		default:
			if !k.NumOK || k.By != "number" {
				t.Errorf("%s key = %+v; want a number", e.Title, k)
			}
		}
	}
	res, err = Run(ix, parsed("sort:kind"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if k := res.SortKey(res.Groups[0].Tasks[0]); k.Class != 1 || k.Text != "task" {
		t.Errorf("kind key = %+v; want work role", k)
	}
}

func TestErrorUnknown(t *testing.T) {
	ix := sortFixture(t)
	tests := []struct {
		q       string
		unknown bool
	}{
		{"nope:x", true},
		{"nope>3", true},
		{"sort:nope", true},
		{"group:nope", true},
		{"kind:nope", false},
		{"status:nope", false},
		{"priority:nope", false},
		{"sort:label", false},
		{"title>3", true},
		{"priority>3", false},
		{"sort:due sort:title", false},
	}
	for _, tt := range tests {
		q, err := Parse(tt.q)
		if err != nil {
			t.Fatal(err)
		}
		_, err = Run(ix, q, Options{})
		var qe *Error
		if !errors.As(err, &qe) {
			t.Errorf("%q: err = %v; want a query error", tt.q, err)
			continue
		}
		if qe.Unknown != tt.unknown {
			t.Errorf("%q: Unknown = %v; want %v (%v)", tt.q, qe.Unknown, tt.unknown, err)
		}
	}
}
