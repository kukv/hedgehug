// Package plural writes a count with its noun in English: "1 task", "2 tasks".
// Every message that has a number in it goes through here.
package plural

import "strconv"

// Count is n and noun, with an s after the noun unless n is 1.
func Count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

// Pick is one when n is 1 and many otherwise, for the word that agrees with a
// count: Pick(n, "has", "have").
func Pick(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
