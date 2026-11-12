// Package rank は、兄弟の並び順の値を持つ。値は英数字 62 文字の文字列で、辞書順に並ぶ。
package rank

import "strings"

const digits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// First は、兄弟がないときの値。
const First = "V"

// After は、s よりあとに並ぶ値を返す。s が空なら First。最後の文字を 1 つ進め、進められなければ First を足す。
func After(s string) string {
	if s == "" {
		return First
	}
	i := strings.IndexByte(digits, s[len(s)-1])
	if i >= 0 && i < len(digits)-1 {
		return s[:len(s)-1] + string(digits[i+1])
	}
	return s + First
}
