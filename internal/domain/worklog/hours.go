// Package worklog は、作業時間の行の決め方と、時間の検査・表記を持つ(02「作業時間」)。
package worklog

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/refusal"
)

const (
	msgEmpty = "時間を入力してください"
	msgScale = "時間は小数第 2 位までで入力してください"
)

// decimal は、10 進の書き方だけを受ける(符号・指数・カンマは数として扱わない)。
var decimal = regexp.MustCompile(`^(\d*)(?:\.(\d*))?$`)

// maxWholeDigits は、整数部の桁数の上限。100 分の 1 時間にして 2^53 を超えないようにする(仕様に上限はない。
// 実際に届く値ではない)。
const maxWholeDigits = 9

// ParseHours は、時間の入力を 100 分の 1 時間の整数にする(A4a-9)。空・数でない・0 以下は「時間を入力してください」、
// 小数第 3 位以下に 0 でない数字があれば「時間は小数第 2 位までで入力してください」(末尾の 0 は数えない。1.250 は 1.25)。
// 文字列のまま見るので、浮動小数点の丸めに左右されない(A4a-26)。
func ParseHours(s string) (int, error) {
	m := decimal.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil || (m[1] == "" && m[2] == "") {
		return 0, refusal.New(msgEmpty)
	}
	whole := strings.TrimLeft(m[1], "0")
	frac := strings.TrimRight(m[2], "0")
	if len(frac) > 2 {
		return 0, refusal.New(msgScale)
	}
	if len(whole) > maxWholeDigits {
		return 0, refusal.New(msgEmpty)
	}
	w, _ := strconv.Atoi("0" + whole)
	f, _ := strconv.Atoi((frac + "00")[:2])
	if h := w*100 + f; h > 0 {
		return h, nil
	}
	return 0, refusal.New(msgEmpty)
}

// Format は、100 分の 1 時間の整数を、小数第 2 位まで・末尾の 0 を省いた表記にする(0.3・1.25・2)。
// 「 時間」は付けない。
func Format(hundredths int) string {
	whole, frac := hundredths/100, hundredths%100
	if frac == 0 {
		return strconv.Itoa(whole)
	}
	return fmt.Sprintf("%d.%s", whole, strings.TrimRight(fmt.Sprintf("%02d", frac), "0"))
}
