package model

import (
	"strings"
	"testing"
)

func TestBetween(t *testing.T) {
	t.Parallel()
	z26, z27 := strings.Repeat("0", 26), strings.Repeat("0", 27)
	zz := strings.Repeat("z", 26)
	for _, tc := range []struct{ a, b, want string }{
		{"", "", "a0"},
		{"", "a0", "Zz"},
		{"", "Zz", "Zy"},
		{"a0", "", "a1"},
		{"a1", "", "a2"},
		{"a0", "a1", "a0V"},
		{"a1", "a2", "a1V"},
		{"a0V", "a1", "a0l"},
		{"Zz", "a0", "ZzV"},
		{"Zz", "a1", "a0"},
		{"", "Y00", "Xzzz"},
		{"bzz", "", "c000"},
		{"a0", "a0V", "a0G"},
		{"a0", "a0G", "a08"},
		{"b125", "b129", "b127"},
		{"a0", "a1V", "a1"},
		{"Zz", "a01", "a0"},
		{"", "a0V", "a0"},
		{"", "b999", "b99"},
		{"", "A" + z26 + "1", "A" + z27 + "V"},
		{"z" + zz[:25] + "y", "", "z" + zz},
		{"z" + zz, "", "z" + zz + "V"},
	} {
		got, err := Between(Rank(tc.a), Rank(tc.b))
		if err != nil || string(got) != tc.want {
			t.Errorf("Between(%q, %q) = %q, %v; want %q", tc.a, tc.b, got, err, tc.want)
		}
	}
}

func TestBetweenErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ a, b string }{
		{"a00", ""}, // trailing zero
		{"a00", "a1"},
		{"0", "1"},   // bad head
		{"a1", "a0"}, // a >= b
		{"a1", "a1"},
		{"", "A" + strings.Repeat("0", 26)}, // smallest integer
		{"a0!", ""},                         // not base62
		{"b1", ""},                          // integer part too short
	} {
		if got, err := Between(Rank(tc.a), Rank(tc.b)); err == nil {
			t.Errorf("Between(%q, %q) = %q, want an error", tc.a, tc.b, got)
		}
	}
}

func TestNBetween(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		a, b string
		n    int
		want string
	}{
		{"", "", 5, "a0 a1 a2 a3 a4"},
		{"a4", "", 10, "a5 a6 a7 a8 a9 aA aB aC aD aE"},
		{"", "a0", 5, "Zv Zw Zx Zy Zz"},
		{"a0", "a1", 0, ""},
	} {
		got, err := NBetween(Rank(tc.a), Rank(tc.b), tc.n)
		if err != nil {
			t.Fatalf("NBetween(%q, %q, %d): %v", tc.a, tc.b, tc.n, err)
		}
		s := make([]string, len(got))
		for i, r := range got {
			s[i] = string(r)
		}
		if strings.Join(s, " ") != tc.want {
			t.Errorf("NBetween(%q, %q, %d) = %v, want %s", tc.a, tc.b, tc.n, s, tc.want)
		}
	}
	// Between two neighbours, every key is strictly inside and increasing.
	got, err := NBetween("a0", "a1", 20)
	if err != nil || len(got) != 20 {
		t.Fatalf("NBetween = %v, %v", got, err)
	}
	prev := Rank("a0")
	for _, r := range got {
		if r <= prev || r >= "a1" {
			t.Fatalf("NBetween out of order: %v", got)
		}
		prev = r
	}
}

func TestRankValidate(t *testing.T) {
	t.Parallel()
	if err := Rank("a0V").Validate(); err != nil {
		t.Error(err)
	}
	if err := Rank("a00").Validate(); err == nil {
		t.Error("Validate accepted a trailing zero")
	}
}
