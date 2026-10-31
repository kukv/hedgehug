package model

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v4"
)

// Rank orders siblings. It is a key in the format of
// rocicorp/fractional-indexing with base62 digits: one head character that
// gives the length of the integer part, then the integer and fraction
// digits. Comparing ranks as strings gives their order, and a new rank can
// always be made between two others without touching them.
type Rank string

const rankDigits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var smallestInteger = "A" + strings.Repeat("0", 26)

// Validate reports whether r is a well-formed rank.
func (r Rank) Validate() error { return validateKey(string(r)) }

// UnmarshalYAML reads the scalar as written, so no rank is taken for another
// type.
func (r *Rank) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.ScalarNode {
		return fmt.Errorf("line %d: want a rank", n.Line)
	}
	*r = Rank(n.Value)
	return nil
}

// Between returns a rank strictly between a and b. An empty a means before
// everything, an empty b after everything.
func Between(a, b Rank) (Rank, error) {
	sa, sb := string(a), string(b)
	if sa != "" {
		if err := validateKey(sa); err != nil {
			return "", err
		}
	}
	if sb != "" {
		if err := validateKey(sb); err != nil {
			return "", err
		}
	}
	if sa != "" && sb != "" && sa >= sb {
		return "", fmt.Errorf("rank %q is not before %q", sa, sb)
	}
	if sa == "" {
		if sb == "" {
			return "a0", nil
		}
		ib := integerPart(sb)
		fb := sb[len(ib):]
		if ib == smallestInteger {
			m, err := midpoint("", fb)
			return Rank(ib + m), err
		}
		if ib < sb {
			return Rank(ib), nil
		}
		r, ok := decrementInteger(ib)
		if !ok {
			return "", errors.New("rank: cannot make a rank before the smallest one")
		}
		return Rank(r), nil
	}
	ia := integerPart(sa)
	fa := sa[len(ia):]
	if sb == "" {
		i, ok := incrementInteger(ia)
		if !ok {
			m, err := midpoint(fa, "")
			return Rank(ia + m), err
		}
		return Rank(i), nil
	}
	ib := integerPart(sb)
	fb := sb[len(ib):]
	if ia == ib {
		m, err := midpoint(fa, fb)
		return Rank(ia + m), err
	}
	i, ok := incrementInteger(ia)
	if !ok {
		return "", errors.New("rank: cannot make a rank after the largest one")
	}
	if i < sb {
		return Rank(i), nil
	}
	m, err := midpoint(fa, "")
	return Rank(ia + m), err
}

// NBetween returns n ranks in increasing order, strictly between a and b.
func NBetween(a, b Rank, n int) ([]Rank, error) {
	switch {
	case n <= 0:
		return nil, nil
	case n == 1:
		r, err := Between(a, b)
		return []Rank{r}, err
	case b == "":
		out := make([]Rank, 0, n)
		c := a
		for range n {
			var err error
			if c, err = Between(c, b); err != nil {
				return nil, err
			}
			out = append(out, c)
		}
		return out, nil
	case a == "":
		out := make([]Rank, n)
		c := b
		for i := n - 1; i >= 0; i-- {
			var err error
			if c, err = Between(a, c); err != nil {
				return nil, err
			}
			out[i] = c
		}
		return out, nil
	}
	mid := n / 2
	c, err := Between(a, b)
	if err != nil {
		return nil, err
	}
	left, err := NBetween(a, c, mid)
	if err != nil {
		return nil, err
	}
	right, err := NBetween(c, b, n-mid-1)
	if err != nil {
		return nil, err
	}
	return append(append(left, c), right...), nil
}

// midpoint returns a fraction strictly between a and b, where an empty b
// means 1. Neither may end in the zero digit.
func midpoint(a, b string) (string, error) {
	if b != "" && a >= b {
		return "", fmt.Errorf("rank: %q is not before %q", a, b)
	}
	if strings.HasSuffix(a, "0") || strings.HasSuffix(b, "0") {
		return "", errors.New("rank: trailing zero")
	}
	if b != "" {
		n := 0
		for digitAt(a, n) == b[n] {
			n++
		}
		if n > 0 {
			m, err := midpoint(tail(a, n), b[n:])
			return b[:n] + m, err
		}
	}
	da := 0
	if a != "" {
		da = strings.IndexByte(rankDigits, a[0])
	}
	db := len(rankDigits)
	if b != "" {
		db = strings.IndexByte(rankDigits, b[0])
	}
	if db-da > 1 {
		return string(rankDigits[(da+db+1)/2]), nil
	}
	if len(b) > 1 {
		return b[:1], nil
	}
	m, err := midpoint(tail(a, 1), "")
	return string(rankDigits[da]) + m, err
}

// digitAt returns s[i], or the zero digit past the end of s.
func digitAt(s string, i int) byte {
	if i < len(s) {
		return s[i]
	}
	return '0'
}

func tail(s string, n int) string {
	if n >= len(s) {
		return ""
	}
	return s[n:]
}

// integerLength returns the length of the integer part that head starts.
func integerLength(head byte) (int, error) {
	switch {
	case head >= 'a' && head <= 'z':
		return int(head-'a') + 2, nil
	case head >= 'A' && head <= 'Z':
		return int('Z'-head) + 2, nil
	}
	return 0, fmt.Errorf("rank: invalid head %q", head)
}

// integerPart returns the integer part of a key that validateKey accepted.
func integerPart(key string) string {
	l, _ := integerLength(key[0])
	return key[:l]
}

func validateKey(key string) error {
	if key == "" {
		return errors.New("rank: empty")
	}
	if key == smallestInteger {
		return fmt.Errorf("rank: %q is reserved", key)
	}
	l, err := integerLength(key[0])
	if err != nil {
		return err
	}
	if l > len(key) {
		return fmt.Errorf("rank: %q is too short", key)
	}
	for i := 1; i < len(key); i++ {
		if strings.IndexByte(rankDigits, key[i]) < 0 {
			return fmt.Errorf("rank: %q has a character outside base62", key)
		}
	}
	if strings.HasSuffix(key[l:], "0") {
		return fmt.Errorf("rank: %q ends in a zero", key)
	}
	return nil
}

func incrementInteger(x string) (string, bool) {
	head, digs := x[0], []byte(x[1:])
	carry := true
	for i := len(digs) - 1; carry && i >= 0; i-- {
		d := strings.IndexByte(rankDigits, digs[i]) + 1
		if d == len(rankDigits) {
			digs[i] = '0'
		} else {
			digs[i] = rankDigits[d]
			carry = false
		}
	}
	if !carry {
		return string(head) + string(digs), true
	}
	switch head {
	case 'Z':
		return "a0", true
	case 'z':
		return "", false
	}
	h := head + 1
	if h > 'a' {
		digs = append(digs, '0')
	} else {
		digs = digs[:len(digs)-1]
	}
	return string(h) + string(digs), true
}

func decrementInteger(x string) (string, bool) {
	head, digs := x[0], []byte(x[1:])
	borrow := true
	for i := len(digs) - 1; borrow && i >= 0; i-- {
		d := strings.IndexByte(rankDigits, digs[i]) - 1
		if d == -1 {
			digs[i] = 'z'
		} else {
			digs[i] = rankDigits[d]
			borrow = false
		}
	}
	if !borrow {
		return string(head) + string(digs), true
	}
	switch head {
	case 'a':
		return "Zz", true
	case 'A':
		return "", false
	}
	h := head - 1
	if h < 'Z' {
		digs = append(digs, 'z')
	} else {
		digs = digs[:len(digs)-1]
	}
	return string(h) + string(digs), true
}

// RankKey is what orders a task among its siblings: its rank, then its id,
// then its ULID.
type RankKey struct {
	ID   string
	ULID ULID
	Rank Rank
}

// RankKey returns what orders t among its siblings.
func (t *Task) RankKey() RankKey { return RankKey{ID: t.ID, ULID: t.ULID, Rank: t.Rank} }

// CompareRankKeys orders siblings: by rank, then id, then ULID.
func CompareRankKeys(a, b RankKey) int {
	if c := cmp.Compare(a.Rank, b.Rank); c != 0 {
		return c
	}
	if c := CompareIDs(a.ID, b.ID); c != 0 {
		return c
	}
	return cmp.Compare(a.ULID.String(), b.ULID.String())
}

// RankAmong returns the rank for self among sibs, which are its live
// siblings. Siblings with the same rank are ordered by id, so when one has
// the rank of self, self gets a rank of its own: the first of the group
// goes between the next smaller rank of the siblings and the group's, any
// other between the group's and the next larger. The order against the
// siblings outside the group stays. Without a sibling of the same rank it
// returns self.Rank.
func RankAmong(self RankKey, sibs []RankKey) (Rank, error) {
	var lower, higher Rank
	group := []RankKey{self}
	for _, o := range sibs {
		switch {
		case o.Rank == self.Rank:
			group = append(group, o)
		case o.Rank < self.Rank && o.Rank > lower:
			lower = o.Rank
		case o.Rank > self.Rank && (higher == "" || o.Rank < higher):
			higher = o.Rank
		}
	}
	if len(group) == 1 {
		return self.Rank, nil
	}
	if slices.MinFunc(group, CompareRankKeys) == self {
		return Between(lower, self.Rank)
	}
	return Between(self.Rank, higher)
}
