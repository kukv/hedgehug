package gitstore

import (
	"context"
	"log/slog"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/infra/gitstore/saves"
)

// baseline は、判定の記録がない Store なら、同期を始めたときの自分の先端から辿れる保存を、通知なしで判定済みに
// する(決定 C4b1-3)。受け取る前に一度だけ呼ぶ。失敗はログに残すだけ(記録がなければ judge が渡さない)。
func (s syncRun) baseline(ctx context.Context) {
	if s.r.Judge == nil || s.r.Judged == nil {
		return
	}
	if err := saves.Baseline(ctx, s.repo, s.id, s.r.Judged, s.start); err != nil {
		slog.Warn("同期の前の保存を判定済みにできませんでした", "store", s.root, "error", err)
	}
}

// finish は、受け取った保存などを判定し(失敗はログに残すだけで、同期の結果は変えない。決定 C4b1-5)、結果を数える。
func (s syncRun) finish(ctx context.Context, theirs, now plumbing.Hash, sent bool) (port.SyncResult, error) {
	if err := s.judge(ctx, theirs); err != nil {
		slog.Warn("受け取った保存を判定できませんでした", "store", s.root, "error", err)
	}
	return s.result(theirs, now, sent)
}

// judge は、remote の先端 theirs から受け取った保存と、このマシンの修復の保存と、消えたものがある合わせを、判定の
// 差し込み口に渡し、自分の先端までを判定済みにする。閲覧のみの同期なら ReadOnly を渡す(決定 C4b2-4)。
func (s syncRun) judge(ctx context.Context, theirs plumbing.Hash) error {
	if s.r.Judge == nil || s.r.Judged == nil {
		return nil
	}
	head, err := s.repo.Head()
	if err != nil {
		return err
	}
	read := func(fn func(r port.Reader) error) error { return fn(&view{root: s.root}) }
	return saves.Judge(ctx, s.repo, s.id, s.r.Judged, s.r.Judge, theirs, head.Hash(), s.receiveOnly, read)
}
