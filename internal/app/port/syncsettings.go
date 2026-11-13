package port

import (
	"context"
	"time"
)

// SyncTiming は、設定の「一般」の「裏で自動的に同期する」と「同期のタイミング」(06「自動で同期する」)。
type SyncTiming struct {
	Auto       bool          // 裏で自動的に同期する
	Interval   time.Duration // 間隔
	AfterWrite time.Duration // 書き込みのあと同期するまで

	GitHubEvery time.Duration // GitHub を読む間隔(07「GitHub を読む間隔」。D4a-7)
}

// SyncSettings は、同期のタイミングを読む口(このマシンだけの設定)。
type SyncSettings interface {
	// Timing は、いまの設定。書かれていない項目は初期値(オン・5 分・30 秒。GitHub は 5 分)。
	Timing(ctx context.Context) (SyncTiming, error)
}
