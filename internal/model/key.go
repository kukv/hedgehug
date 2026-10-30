package model

import (
	"fmt"
	"regexp"
	"strconv"
)

var (
	prefixRE = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,9}$`)
	keyRE    = regexp.MustCompile(`^([A-Z][A-Z0-9]{1,9})-([1-9][0-9]*)$`)
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
