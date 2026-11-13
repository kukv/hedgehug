package githublink

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
)

// readEvery は、ReadOnce を呼ぶ最短の間。ReadOnce はすべての Store のアイテムとログを Store の排他の中で読むので、
// 1 秒ごとには呼ばない。読む時期はリポジトリごとに ReadOnce が決めるので、1 分ごとに見れば足りる。
const readEvery = time.Minute

// Reader は、GitHub を自動で読む(07「GitHub を読む間隔」)。
type Reader struct {
	Links    Links
	Settings port.SyncSettings
	Waiting  func() bool // cmd で syncing.Auto.Waiting を渡す

	read    time.Time // 前回に ReadOnce を呼んだ時刻(ゼロなら、まだ)
	checked time.Time // 前回に確かめ直した時刻(ゼロなら、まだ)
}

// Step は、自動で読む条件のときだけ、確かめ直し(間隔ごとに 1 回)と ReadOnce を行う。
// 待つ条件のあいだに時期が来たものは、解けた最初の Step で行う(「解けたら続けて読む」)。
func (r *Reader) Step(ctx context.Context) error {
	timing, err := r.Settings.Timing(ctx)
	if err != nil {
		return err
	}
	if !timing.Auto || r.Waiting() {
		return nil
	}
	now := r.Links.now()
	var errs []error
	if r.checked.IsZero() || !now.Before(r.checked.Add(timing.GitHubEvery)) {
		r.checked = now // 失敗しても、次の間隔まで確かめ直さない
		back, err := r.Links.Tokens.Recheck(ctx)
		if err != nil {
			errs = append(errs, err)
		}
		if back { // 設定の「アカウントとメンバー」が一覧を読み直す。トーストは出さない(D4a-18)
			r.Links.Events.Publish(port.Event{Kind: port.EventGitHub})
		}
	}
	if r.read.IsZero() || !now.Before(r.read.Add(readEvery)) {
		r.read = now
		errs = append(errs, r.Links.ReadOnce(ctx, timing.GitHubEvery, false))
	}
	return errors.Join(errs...)
}

// Run は、ctx が終わるまで 1 秒ごとに Step を呼ぶ。失敗はログに出して続ける(syncing.Auto.Run と同じ形)。
func (r *Reader) Run(ctx context.Context) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		if err := r.Step(ctx); err != nil {
			slog.Warn("GitHub の自動の読み取りに失敗しました", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
