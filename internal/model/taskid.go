package model

import (
	"cmp"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	prefixRE = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,9}$`)
	idRE     = regexp.MustCompile(`^([A-Z][A-Z0-9]{1,9})-([1-9][0-9]*)$`)
	// idWordRE finds words of the form of an id anywhere in a text.
	idWordRE = regexp.MustCompile(`\b([A-Z][A-Z0-9]{1,9})-([1-9][0-9]*)\b`)
)

// ValidIDPrefix checks an id prefix: 2 to 10 upper-case letters and
// digits, starting with a letter.
func ValidIDPrefix(p string) error {
	if !prefixRE.MatchString(p) {
		return fmt.Errorf("%q is not an id prefix: use 2 to 10 upper-case letters and digits, starting with a letter", p)
	}
	return nil
}

// ParseTaskID splits an id such as WEB-42 into its prefix and number.
func ParseTaskID(id string) (prefix string, n int, ok bool) {
	m := idRE.FindStringSubmatch(id)
	if m == nil {
		return "", 0, false
	}
	n, err := strconv.Atoi(m[2])
	if err != nil {
		return "", 0, false
	}
	return m[1], n, true
}

// FormatTaskID makes the id of number n under prefix.
func FormatTaskID(prefix string, n int) string { return prefix + "-" + strconv.Itoa(n) }

// IDNumbersIn returns, for each id prefix, the largest number of the words
// of data that have the form of an id. It is for a task file that does not
// decode, whose ids cannot be told from the rest: counting a word that is
// not an id only skips numbers, which is safe.
func IDNumbersIn(data []byte) map[string]int {
	out := map[string]int{}
	for _, m := range idWordRE.FindAllSubmatch(data, -1) {
		if n, err := strconv.Atoi(string(m[2])); err == nil && n > out[string(m[1])] {
			out[string(m[1])] = n
		}
	}
	return out
}

// CompareIDs orders ids by prefix, then by number. What is not the form of
// an id comes after the ids, in string order.
func CompareIDs(a, b string) int {
	pa, na, oka := ParseTaskID(a)
	pb, nb, okb := ParseTaskID(b)
	switch {
	case oka && okb:
		if c := strings.Compare(pa, pb); c != 0 {
			return c
		}
		return cmp.Compare(na, nb)
	case oka:
		return -1
	case okb:
		return 1
	}
	return strings.Compare(a, b)
}
