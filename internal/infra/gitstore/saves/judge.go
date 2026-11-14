package saves

import (
	"context"
	"errors"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/port"
)

// Baseline は、store の判定の記録がなければ、from から辿れる保存を、通知なしで判定済みにする(決定 C4b1-3)。
// 同期を始めたとき(受け取る前)に、自分の先端を from に渡す。
func Baseline(ctx context.Context, repo *git.Repository, store port.StoreID, judged port.JudgedSaves, from plumbing.Hash) error {
	started, err := judged.Started(ctx, store)
	if err != nil || started {
		return err
	}
	return mark(ctx, repo, store, judged, from)
}

// Judge は、remote の先端 theirs から辿れて判定していない保存を judge に渡し、そのあと自分の先端 head から
// 辿れる保存をすべて判定済みにする(決定 C4b1-2)。judge がエラーを返したら、受け取った保存は判定済みにせず
// (決定 C4b1-5)、このマシンの保存だけを判定済みにする(markOwn)。
// 記録がまだない(Baseline が失敗した)ときは、judge に渡さずに判定済みにだけする(履歴のすべてを渡さないため)。
func Judge(ctx context.Context, repo *git.Repository, store port.StoreID, judged port.JudgedSaves, judge port.JudgeFunc,
	theirs, head plumbing.Hash, read func(fn func(r port.Reader) error) error,
) error {
	started, err := judged.Started(ctx, store)
	if err != nil {
		return err
	}
	if started {
		received, err := Unjudged(repo, theirs, knownIn(ctx, store, judged))
		if err != nil {
			return err
		}
		if len(received) > 0 {
			if err := judge(ctx, store, received, read); err != nil {
				return errors.Join(err, markOwn(ctx, repo, store, judged, theirs, head))
			}
		}
	}
	return mark(ctx, repo, store, judged, head)
}

// mark は、from から辿れて判定していない保存を、すべて判定済みにする。
func mark(ctx context.Context, repo *git.Repository, store port.StoreID, judged port.JudgedSaves, from plumbing.Hash) error {
	keys, err := Reachable(repo, from, knownIn(ctx, store, judged))
	if err != nil || len(keys) == 0 {
		return err
	}
	return judged.Mark(ctx, store, keys)
}

// markOwn は、head から辿れて theirs から辿れない、Hedgehug-Op を持つ保存(このマシンの保存と修復)を判定済みに
// する。送ったあとに remote から戻ってきても judge に渡さないため。合わせは判定済みにしない(次に辿るとき、合わせを
// 通り抜けて、判定していない受け取った保存へ届く)。判定していない受け取った保存より後の保存も判定済みにしない
// (そこで辿るのが止まり、受け取った保存へ届かなくなるため)。
func markOwn(ctx context.Context, repo *git.Repository, store port.StoreID, judged port.JudgedSaves, theirs, head plumbing.Hash) error {
	known := knownIn(ctx, store, judged)
	pending, err := walk(repo, theirs, known) // 判定していない受け取った保存(と、その合わせ)
	if err != nil {
		return err
	}
	mine, err := walk(repo, head, known)
	if err != nil {
		return err
	}
	var keys []port.SaveKey
	for h, n := range mine {
		if n.key.Op != "" && !after(mine, pending, h) {
			keys = append(keys, n.key)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	return judged.Mark(ctx, store, keys)
}

// after は、h が pending のどれかか、mine の中の親を辿って pending のどれかに届くか。
func after(mine, pending map[plumbing.Hash]node, h plumbing.Hash) bool {
	seen := map[plumbing.Hash]bool{}
	stack := []plumbing.Hash{h}
	for len(stack) > 0 {
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if _, ok := pending[top]; ok {
			return true
		}
		n, ok := mine[top]
		if !ok || seen[top] {
			continue
		}
		seen[top] = true
		stack = append(stack, n.c.ParentHashes...)
	}
	return false
}

func knownIn(ctx context.Context, store port.StoreID, judged port.JudgedSaves) func(port.SaveKey) (port.Match, error) {
	return func(k port.SaveKey) (port.Match, error) { return judged.Known(ctx, store, k) }
}
