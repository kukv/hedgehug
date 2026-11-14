package search

import "context"

// ScreenStates は、画面ごとの状態(JSON の文字列)の保存先(E1-16。state.db)。中身は見ない。
// 画面の鍵は "search" など。F3 の appreports.Selections と同じく、port が満杯なのでここに置く。
type ScreenStates interface {
	// Get は、保存した文字列を返す。なければ 2 つ目の値が false。
	Get(ctx context.Context, screen string) (string, bool, error)
	// Put は、文字列を置く(あれば上書きする)。
	Put(ctx context.Context, screen, body string) error
}
