package model

import (
	"fmt"
	"regexp"
	"strconv"
)

var (
	prefixRE = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,9}$`)
	keyRE    = regexp.MustCompile(`^([A-Z][A-Z0-9]{1,9})-([1-9][0-9]*)$`)
	// keyWordRE finds words of the form of a key anywhere in a text.
	keyWordRE = regexp.MustCompile(`\b([A-Z][A-Z0-9]{1,9})-([1-9][0-9]*)\b`)
)

// ValidKeyPrefix checks a key prefix: 2 to 10 upper-case letters and
// digits, starting with a letter.
func ValidKeyPrefix(p string) error {
	if !prefixRE.MatchString(p) {
		return fmt.Errorf("%q is not a key prefix: use 2 to 10 upper-case letters and digits, starting with a letter", p)
	}
	return nil
}

// ParseKey splits a key such as WEB-42 into its prefix and number.
func ParseKey(k string) (prefix string, n int, ok bool) {
	m := keyRE.FindStringSubmatch(k)
	if m == nil {
		return "", 0, false
	}
	n, err := strconv.Atoi(m[2])
	if err != nil {
		return "", 0, false
	}
	return m[1], n, true
}

// FormatKey makes the key of number n under prefix.
func FormatKey(prefix string, n int) string { return prefix + "-" + strconv.Itoa(n) }

// KeyNumbersIn returns, for each key prefix, the largest number of the words
// of data that have the form of a key. It is for a task file that does not
// decode, whose keys cannot be told from the rest: counting a word that is
// not a key only skips numbers, which is safe.
func KeyNumbersIn(data []byte) map[string]int {
	out := map[string]int{}
	for _, m := range keyWordRE.FindAllSubmatch(data, -1) {
		if n, err := strconv.Atoi(string(m[2])); err == nil && n > out[string(m[1])] {
			out[string(m[1])] = n
		}
	}
	return out
}
