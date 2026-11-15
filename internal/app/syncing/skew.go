package syncing

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// Skew は、GitHub の応答で測ったずれ(GitHub の時刻 − このマシンの時計)を受け取り、
// 警告の分が変わったときだけ記録して画面に知らせる(決定 C5d-7・C5d-8)。
type Skew struct {
	Records port.ClockSkews
	Events  port.Events

	mu     sync.Mutex
	loaded bool
	last   int
}

// Observe は、測ったずれを受け取る。応答のたびに、いくつもの goroutine から呼ばれる。
// 記録の読み書きに失敗しても、問い合わせの結果は変えない(ログだけ残す)。
func (s *Skew) Observe(d time.Duration) {
	ctx := context.Background()
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.loaded {
		// 読めなければ 0(警告なし)から続ける。読めたときだけ覚える。
		if v, err := s.Records.Skew(ctx); err != nil {
			slog.Warn("時計のずれの記録を読めませんでした", "error", err)
		} else {
			s.last = v
			s.loaded = true
		}
	}
	m := syncstatus.SkewMinutes(d)
	if m == s.last {
		return
	}
	if err := s.Records.SetSkew(ctx, m); err != nil {
		// 前の値は変えない。次の応答でもう一度書く。
		slog.Warn("時計のずれを記録できませんでした", "error", err)
		return
	}
	s.last = m
	s.Events.Publish(port.Event{Kind: port.EventSync})
}

// Minutes は、記録した警告の分を返す(正は進んでいる、負は遅れている、0 は警告なし)。
func (s *Skew) Minutes(ctx context.Context) (int, error) {
	return s.Records.Skew(ctx)
}
