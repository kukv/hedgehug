// このファイルは、同期のうちロックの中で行う 2 つの段を持つ。受け取った remote の先端を自分の先端に取り込む段
// (integrate・forward・merge・apply・guarded。作業ツリーに書いてコミットする)と、同期の最後に保存を判定して
// 結果を数える段(baseline・finish・judge)。

package gitstore

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
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
// mine は、ロックの中で読んだ自分の先端(送ったなら送った先端、閲覧のみなら合わせたあとの先端)。判定と数え方は、
// いまの先端ではなく mine を基準にする(ロックの外で送るあいだにできた保存は、まだ送っていないので判定しない。決定 G1-10)。
func (s syncRun) finish(ctx context.Context, theirs, mine plumbing.Hash, sent bool) (port.SyncResult, error) {
	if err := s.judge(ctx, theirs, mine); err != nil {
		slog.Warn("受け取った保存を判定できませんでした", "store", s.root, "error", err)
	}
	now := theirs // 同期のあとに remote が持つ先端
	if sent {
		now = mine
	}
	return s.result(theirs, now, sent)
}

// judge は、remote の先端 theirs から受け取った保存と、このマシンの修復の保存と、消えたものがある合わせを、判定の
// 差し込み口に渡し、mine までを判定済みにする。閲覧のみの同期なら ReadOnly を渡す(決定 C4b2-4)。
// 判定の基準は mine だが、read が読む作業ツリーはいまの先端のもので、送るあいだにできた保存を含み、mine より新しいことがある。
func (s syncRun) judge(ctx context.Context, theirs, mine plumbing.Hash) error {
	if s.r.Judge == nil || s.r.Judged == nil {
		return nil
	}
	read := func(fn func(r port.Reader) error) error { return fn(&view{root: s.root}) }
	return saves.Judge(ctx, s.repo, s.id, s.r.Judged, s.r.Judge, theirs, mine, s.receiveOnly, read)
}

// integrate は、自分の先端に remote の先端 theirs を取り込む。
func (s syncRun) integrate(theirs plumbing.Hash) error {
	head, err := s.repo.Head()
	if err != nil {
		return err
	}
	ours := head.Hash()
	oc, err := s.repo.CommitObject(ours)
	if err != nil {
		return err
	}
	tc, err := s.repo.CommitObject(theirs)
	if err != nil {
		return err
	}
	if ours == theirs {
		return nil
	}
	if behind, err := tc.IsAncestor(oc); err != nil || behind {
		return err
	}
	wt, err := s.repo.Worktree()
	if err != nil {
		return err
	}
	if ahead, err := oc.IsAncestor(tc); err != nil {
		return err
	} else if ahead {
		return s.guarded(func() error { return s.forward(wt, ours, theirs) })
	}
	bases, err := oc.MergeBase(tc)
	if err != nil {
		return err
	}
	if len(bases) == 0 {
		return port.ErrNoCommonHistory
	}
	return s.guarded(func() error { return s.merge(wt, bases[0].Hash, ours, theirs) })
}

// guarded は、作業ツリーを変える fn のあいだ、保存の途中であることを示す印を置く。プロセスが落ちても、
// 次の操作が印を見て、いまのコミットに戻す(保存形式「技術構成」)。fn が失敗したときは印を残し、
// 次の操作に戻させる(戻すのは何度行っても同じ結果になる)。
func (s syncRun) guarded(fn func() error) error {
	if err := os.WriteFile(markerPath(s.root), nil, 0o600); err != nil {
		return err
	}
	if err := fn(); err != nil {
		return err
	}
	// 作業ツリーはコミットと同じ。印が消せなくても、次の操作がいまのコミットに戻すだけで、何も失われない。
	_ = os.Remove(markerPath(s.root))
	return nil
}

// forward は、自分の先端を remote の先端 theirs へ進める。ハードリセットは使わない(go-git v5 のハードリセットは
// インデックスにないファイルも消すので、Store のものでないファイルまで消してしまう)。
func (s syncRun) forward(wt *git.Worktree, ours, theirs plumbing.Hash) error {
	from, err := readTree(s.repo, ours)
	if err != nil {
		return err
	}
	to, err := readTree(s.repo, theirs)
	if err != nil {
		return err
	}
	return s.apply(wt, ours, from, to, func() error {
		return s.repo.Storer.SetReference(plumbing.NewHashReference(s.branch, theirs))
	})
}

// merge は、3 つのツリーを合わせて作業ツリーに書き、2 つの親を持つコミットにする。
func (s syncRun) merge(wt *git.Worktree, base, ours, theirs plumbing.Hash) error {
	trees := make([]tree, 3)
	for i, h := range []plumbing.Hash{base, ours, theirs} {
		t, err := readTree(s.repo, h)
		if err != nil {
			return err
		}
		trees[i] = t
	}
	merged, err := mergeTrees(trees[0], trees[1], trees[2])
	if err != nil {
		return err
	}
	return s.apply(wt, ours, trees[1], merged, func() error {
		sig := signature(saves.BotName, botEmail, s.r.stores.clock.Now())
		// 結果が自分の版と同じ(相手が書いたのが、こちらで完全に削除したものだけ)でも、合わせを保存する(決定 C4b2-15)。
		opts := &git.CommitOptions{Author: sig, Committer: sig, Parents: []plumbing.Hash{ours, theirs}, AllowEmptyCommits: true}
		_, err := wt.Commit("remote と合わせる\n", opts)
		return err
	})
}

// apply は、作業ツリーを from から to にし、変えたファイルだけをインデックスに載せて、finish でブランチを動かす。
// 失敗したら ours に戻し、新しく作ったファイルを消す。
func (s syncRun) apply(wt *git.Worktree, ours plumbing.Hash, from, to tree, finish func() error) error {
	changes, err := writeTree(s.root, from, to)
	if err == nil {
		err = stage(wt, changes)
	}
	if err == nil {
		err = finish()
	}
	if err != nil {
		errs := []error{err, wt.Reset(&git.ResetOptions{Commit: ours, Mode: git.HardReset})}
		for _, p := range changes.created {
			errs = append(errs, os.Remove(filepath.Join(s.root, p)))
		}
		return errors.Join(errs...)
	}
	return nil
}
