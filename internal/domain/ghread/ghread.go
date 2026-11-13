// Package ghread は、GitHub を読む間隔と利用上限の規則(07「GitHub を読む間隔」
// 「GitHub との通信に失敗したとき」)を持つ。時計は呼び出し側から受け取る。
package ghread

import "time"

// Record は、Store のリポジトリ 1 つの、前回の読み取りの記録。ゼロ値は「まだ読んでいない」。
type Record struct {
	Attempt   time.Time // 前回に試した時刻(このマシンの時計)
	Failed    bool      // 前回は失敗した
	Limit     int       // 利用上限(0 は分からない)
	Remaining int       // 利用上限の残り
	ResetAt   time.Time // 上限が戻る時刻
	NotBefore time.Time // これより前は読まない(利用上限で断られたとき)
}

// Low は、残りが上限の 10% 未満で、上限が戻る時刻が未来のとき真。このとき間隔は 3 倍になる。
func Low(now time.Time, r Record) bool {
	return r.Limit > 0 && r.Remaining*10 < r.Limit && now.Before(r.ResetAt)
}

// Due は、いま読んでよいとき真。読まない期限を過ぎていて、前回に試してから間隔
// (Low のときは 3 倍)が過ぎている。成功でも失敗でも試した時刻で数える。
func Due(now time.Time, r Record, every time.Duration) bool {
	wait := every
	if Low(now, r) {
		wait *= 3
	}
	return !now.Before(r.NotBefore) && (r.Attempt.IsZero() || now.Sub(r.Attempt) >= wait)
}

// Limited は、利用上限で断られたときの、読まない期限。
// 上限が戻る時刻が分かればその時刻、分からなければ 1 分後。
func Limited(now, reset time.Time) time.Time {
	if reset.IsZero() {
		return now.Add(time.Minute)
	}
	return reset
}
