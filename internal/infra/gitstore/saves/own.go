package saves

import (
	"context"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// ownSaves は、このマシンで作った保存(決定 C4c1-4)を、印ごとに返す。自分の先端 head から辿れて remote の先端
// theirs から辿れない、Hedgehug-Op を持つ、同期の修復でない保存(まだ送っていない保存)。辿るのは theirs から
// 辿れる保存で止め、判定済みかどうかでは止めない(最初の印付けが判定済みにした保存も数える)。
func ownSaves(repo *git.Repository, theirs, head plumbing.Hash) (map[plumbing.Hash]port.SaveKey, error) {
	if head == theirs {
		return map[plumbing.Hash]port.SaveKey{}, nil // 送っていない保存はない(履歴を辿らない)
	}
	received, err := walk(repo, theirs, func(port.SaveKey) (port.Match, error) { return port.NotJudged, nil })
	if err != nil {
		return nil, err
	}
	mine, err := walk(repo, head, func(k port.SaveKey) (port.Match, error) {
		if _, ok := received[plumbing.NewHash(k.Version)]; ok {
			return port.SameVersion, nil
		}
		return port.NotJudged, nil
	})
	if err != nil {
		return nil, err
	}
	out := map[plumbing.Hash]port.SaveKey{}
	for h, n := range mine {
		if n.key.Op == "" {
			continue
		}
		s, err := saveOf(n) // 同期の修復かは Pending と同じ見分け方で
		if err != nil {
			return nil, err
		}
		if !s.Repair {
			out[h] = n.key
		}
	}
	return out, nil
}

// ownOf は、判定に渡す Received.Own。op が own にあるか、記録でこのマシンの保存か。閲覧のみの同期では常に false。
func ownOf(ctx context.Context, store port.StoreID, judged port.JudgedSaves, own map[plumbing.Hash]port.SaveKey,
	readOnly bool,
) func(op change.ID) (bool, error) {
	ops := map[change.ID]bool{}
	for _, k := range own {
		ops[k.Op] = true
	}
	return func(op change.ID) (bool, error) {
		if readOnly || op == "" {
			return false, nil
		}
		if ops[op] {
			return true, nil
		}
		return judged.Own(ctx, store, op)
	}
}

// markOwnKeys は、own の保存を、このマシンの保存として判定済みにする。
func markOwnKeys(ctx context.Context, store port.StoreID, judged port.JudgedSaves, own map[plumbing.Hash]port.SaveKey) error {
	if len(own) == 0 {
		return nil
	}
	keys := make([]port.SaveKey, 0, len(own))
	for _, k := range own {
		keys = append(keys, k)
	}
	return judged.MarkOwn(ctx, store, keys)
}

// writeMarked は、判定のエラーのあとに判定済みにする keys を書く。own にあるものは MarkOwn、ほかは Mark で書く。
// keys にない own の保存でも、すでに判定済み(SameVersion)のものは、このマシンの保存として覚え直す(最初の印付けが
// 判定済みにした保存。判定済みかどうかは変えない)。
func writeMarked(ctx context.Context, store port.StoreID, judged port.JudgedSaves, keys []port.SaveKey,
	own map[plumbing.Hash]port.SaveKey,
) error {
	var plain, mine []port.SaveKey
	in := map[string]bool{}
	for _, k := range keys {
		in[k.Version] = true
		if _, ok := own[plumbing.NewHash(k.Version)]; ok {
			mine = append(mine, k)
		} else {
			plain = append(plain, k)
		}
	}
	for _, k := range own {
		if in[k.Version] {
			continue
		}
		if m, err := judged.Known(ctx, store, k); err != nil {
			return err
		} else if m == port.SameVersion {
			mine = append(mine, k)
		}
	}
	if len(plain) > 0 {
		if err := judged.Mark(ctx, store, plain); err != nil {
			return err
		}
	}
	if len(mine) == 0 {
		return nil
	}
	return judged.MarkOwn(ctx, store, mine)
}
