package transfer

import (
	"errors"
	"slices"

	"github.com/kukv/hedgehug/internal/domain/item"
)

var (
	// ErrNotFound は、移すアイテムがない。
	ErrNotFound = errors.New("transfer: item not found")
	// ErrStep は、Step は単独で移せない。
	ErrStep = errors.New("transfer: a step cannot be moved")
)

// Cut は、移すものの外のアイテムとの関連(持つ側 From と相手 To)。別の Store へ移すと切れる。
type Cut struct {
	From, To item.ID
	Type     string
}

// Set は、移すもの。Below は削除済みを含む、深さ優先・兄弟の順の子と孫。
type Set struct {
	Root    item.Item
	Below   []item.Item
	Outside []Cut
}

// Contains は、id が移すものの中にいるか。
func (s Set) Contains(id item.ID) bool {
	if s.Root.ID == id {
		return true
	}
	return slices.ContainsFunc(s.Below, func(it item.Item) bool { return it.ID == id })
}

// Collect は、root と下のもの(削除済みを含む)と、外の(all にいる)アイテムとの関連を集める。
// 関連は持つ側と相手の片方だけが中のもので、両方の側から数える。all にいない相手への関連は数えない。
func Collect(root item.ID, all []item.Item) (Set, error) {
	i := slices.IndexFunc(all, func(it item.Item) bool { return it.ID == root })
	if i < 0 {
		return Set{}, ErrNotFound
	}
	if all[i].Kind == item.Step {
		return Set{}, ErrStep
	}
	s := Set{Root: all[i], Below: item.Descendants(root, all)}
	s.Outside = cutsOutside(s.Contains, all)
	return s, nil
}
