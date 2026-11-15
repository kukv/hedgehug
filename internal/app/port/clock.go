package port

import (
	"context"
	"time"
)

// Clock は現在時刻(このマシンの時計。補正しない)。
type Clock interface {
	Now() time.Time
}

// ClockSkews は、GitHub の時刻とこのマシンの時計のずれの警告の分(06「時計のずれの警告」)を覚える。
// 正はこのマシンの時計が進んでいる、負は遅れている、0 は警告なし。
type ClockSkews interface {
	// Skew は、覚えた分を返す。覚えていなければ 0。
	Skew(ctx context.Context) (int, error)
	SetSkew(ctx context.Context, minutes int) error
}
