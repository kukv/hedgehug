// Package rank は、兄弟の並び順の値を持つ。値は英数字 62 文字の文字列で、辞書順に並ぶ。
package rank

import "strings"

const digits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// First は、兄弟がないときの値。
const First = "V"

// After は、s よりあとに並ぶ値を返す。s が空なら First。
// 値は「先頭の z の並び(段) + 数え(先頭は z でない、決まった桁数の 62 進数)」と見る。
// 数えを 1 つ進め(桁数は変えない、'0' で終わるなら '1' にする)、数えが尽きたら z を 1 つ増やし、
// 1 桁広い数えの最初の値にする。末尾に続けても、62 倍ごとに 2 文字しか伸びない。
// 数えがない値(z だけの値)と、digits にない文字を含む値には First を足す。
func After(s string) string {
	k := len(s) - len(strings.TrimLeft(s, "z"))
	if k == len(s) {
		return s + First
	}
	count := []byte(s[k:])
	for i := len(count) - 1; i >= 0; i-- {
		d := strings.IndexByte(digits, count[i])
		if d < 0 { // digits にない文字(手での書き換え)。後ろに足せば必ずあとになる。
			return s + First
		}
		if d < len(digits)-1 {
			count[i] = digits[d+1]
			break
		}
		count[i] = digits[0]
	}
	if count[0] == 'z' { // 数えが尽きた(先頭が z になる)ので、次の段へ。
		return s[:k] + "z" + strings.Repeat("0", len(count)) + "1"
	}
	if count[len(count)-1] == '0' {
		count[len(count)-1] = '1'
	}
	return s[:k] + string(count)
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
