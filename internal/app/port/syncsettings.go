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

// SyncSettings は、同期のタイミングを読み書きする口(このマシンだけの設定)。
type SyncSettings interface {
	// Timing は、いまの設定。書かれていない項目は初期値(オン・5 分・30 秒。GitHub は 5 分)。
	Timing(ctx context.Context) (SyncTiming, error)
	// SetTiming は、sync の 4 つの項目をすべて書く。
	SetTiming(ctx context.Context, t SyncTiming) error
}

// 08 の同期のタイミングの選択肢(G1-18)。ここにない値は、読むときは初期値として扱い、画面からは受け付けない。
var (
	IntervalChoices   = []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, 30 * time.Minute, time.Hour}
	AfterWriteChoices = []time.Duration{10 * time.Second, 30 * time.Second, time.Minute}
	GitHubChoices     = []time.Duration{5 * time.Minute, 15 * time.Minute, 30 * time.Minute, time.Hour}
)
