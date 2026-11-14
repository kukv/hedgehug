package rank_test

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/rank"
)

// 同じ順位の群の中には入らず、群の手前(after なら後ろ)に入る。
func TestAround(t *testing.T) {
	sorted := []string{"V", "W", "W", "X"}
	cases := []struct {
		j     int
		after bool
		want  string
	}{
		{2, false, rank.Between("V", "W")},
		{1, true, rank.Between("W", "X")},
		{3, true, rank.After("X")},
		{0, false, rank.Between("", "V")},
	}
	for _, c := range cases {
		if got := rank.Around(sorted, c.j, c.after); got != c.want {
			t.Errorf("Around(%v, %d, %v) = %q, want %q", sorted, c.j, c.after, got, c.want)
		}
	}
}

func TestAfter(t *testing.T) {
	for in, want := range map[string]string{"": "V", "V": "W", "a0V": "a0W", "9": "A", "Z": "a", "z": "zV", "zz": "zzV", "zV": "zW", "zy": "zz01", "y": "z01", "Vz": "W1", "azz": "b01"} {
		if got := rank.After(in); got != want {
			t.Errorf("After(%q) = %q, want %q", in, got, want)
		}
	}
}

// 前の作りが書いた値(z の並びに First を足した値など)からも、後ろの値を作れる。
func TestAfterFromAnyValue(t *testing.T) {
	for _, in := range []string{"z", "zz", "zV", "zzzW", "Vz", "Az", "azz", "yz", "yzz", "zyzz", "a0V", "V_", "V-z", strings.Repeat("z", 30)} {
		got := rank.After(in)
		if got <= in || strings.HasSuffix(got, "0") {
			t.Errorf("After(%q) = %q, want a later value not ending in 0", in, got)
		}
	}
}

// 末尾に続けても、値の長さは件数の対数でしか伸びない。
func TestAfterStaysShort(t *testing.T) {
	appendMany(t, "", 10000, 6)
	old := strings.Repeat("z", 10)
	appendMany(t, old, 1000, len(old)+4)
}

func appendMany(t *testing.T, s string, n, maxLen int) {
	t.Helper()
	for range n {
		next := rank.After(s)
		if next <= s || strings.HasSuffix(next, "0") {
			t.Fatalf("After(%q) = %q, want a later value not ending in 0", s, next)
		}
		if len(next) > maxLen {
			t.Fatalf("After(%q) = %q is longer than %d", s, next, maxLen)
		}
		s = next
	}
}

func TestBetweenWithCarry(t *testing.T) {
	got := rank.Between("VAz", "W")
	if got <= "VAz" || got >= "W" {
		t.Errorf("Between(VAz, W) = %q, want VAz < value < W", got)
	}
}

func TestBetweenAfterProperty(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	const digits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	for range 5000 {
		b := make([]byte, 1+r.IntN(8))
		for i := range b {
			b[i] = digits[r.IntN(len(digits))]
		}
		if b[len(b)-1] == '0' {
			b[len(b)-1] = '1'
		}
		a := string(b)
		hi := rank.After(a)
		got := rank.Between(a, hi)
		if got <= a || got >= hi || strings.HasSuffix(got, "0") {
			t.Fatalf("Between(%q, %q) = %q", a, hi, got)
		}
	}
}

func TestAfterIsAlwaysLater(t *testing.T) {
	s := ""
	for range 500 {
		next := rank.After(s)
		if next <= s {
			t.Fatalf("After(%q) = %q is not later", s, next)
		}
		s = next
	}
}

func TestBetween(t *testing.T) {
	cases := [][2]string{{"", "V"}, {"V", "W"}, {"V", "X"}, {"Vz", "W"}, {"V", "V1"}, {"", "1"}, {"a", "b"}}
	for _, c := range cases {
		got := rank.Between(c[0], c[1])
		if got <= c[0] || got >= c[1] {
			t.Errorf("Between(%q, %q) = %q", c[0], c[1], got)
		}
	}
	if got := rank.Between("V", ""); got != rank.After("V") {
		t.Errorf("no upper bound: %q", got)
	}
	// 何度あいだに詰めても、順が保たれる。
	lo, hi := "V", "W"
	for range 200 {
		m := rank.Between(lo, hi)
		if m <= lo || m >= hi {
			t.Fatalf("Between(%q, %q) = %q", lo, hi, m)
		}
		hi = m
	}
}
