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

// Between は、a < 値 < b の値。a が空なら b より前のどこか、b が空なら After(a)。
// 呼ぶ側は a < b を守る。値の文字は digits だけで、'0' で終わる値は作らない(作れる値が尽きないため)。
func Between(a, b string) string {
	if b == "" {
		return After(a)
	}
	var out []byte
	for i := 0; ; i++ {
		lo := 0
		if i < len(a) {
			lo = strings.IndexByte(digits, a[i])
		}
		hi := strings.IndexByte(digits, b[i]) // a < b なので、b が尽きる前に lo < hi の桁が来る
		if lo == hi {
			out = append(out, digits[lo])
			continue
		}
		if mid := (lo + hi) / 2; mid > lo {
			return string(append(out, digits[mid]))
		}
		// hi == lo+1: この桁は lo にして、残りは a の続きよりあとにする。
		out = append(out, digits[lo])
		rest := ""
		if i+1 < len(a) {
			rest = a[i+1:]
		}
		return string(out) + After(rest)
	}
}
