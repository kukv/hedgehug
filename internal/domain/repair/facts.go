// Package repair は、同期で合わせたあと、送る前に行う修復の規則を持つ(保存形式「同期の修復」)。
// 合わせた結果を読み、2 台が同じ答えを出すように直す行を決める。git の言葉は使わない。
package repair

import "github.com/kukv/hedgehug/internal/domain/change"

// Facts は、共通の祖先にない変更記録の ID(すべてのログの行)を、自分の側(Ours)と remote の側(Theirs)に
// 分けたもの。まだ送っていない合わせごとに集める(決定 C3a-4)。
type Facts struct {
	Ours, Theirs map[change.ID]bool
}

// Side は、行 id がどちらの側にあるか。自分の側なら 1、remote の側なら 2、どちらでもなければ 0。
func (f Facts) Side(id change.ID) int {
	switch {
	case f.Ours[id]:
		return 1
	case f.Theirs[id]:
		return 2
	}
	return 0
}
