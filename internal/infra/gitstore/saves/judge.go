package saves

import (
	"context"

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
// 辿れる保存をすべて判定済みにする(決定 C4b1-2)。judge がエラーを返したら、何も判定済みにしない(決定 C4b1-5)。
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
				return err
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

func knownIn(ctx context.Context, store port.StoreID, judged port.JudgedSaves) func(port.SaveKey) (port.Match, error) {
	return func(k port.SaveKey) (port.Match, error) { return judged.Known(ctx, store, k) }
}
