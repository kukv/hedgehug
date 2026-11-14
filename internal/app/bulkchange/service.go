// Package bulkchange は、選んだアイテムの項目を 1 つ、まとめて変える(F2b2)。
package bulkchange

import (
	"context"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Service は、一括変更が使う口。
type Service struct {
	Stores    port.Stores
	StoreList port.StoreList
	Identity  port.Identity
	Clock     port.Clock
	Inbox     port.Inbox
}

// Target は、選んだアイテム 1 つ。
type Target struct {
	Store port.StoreID
	ID    item.ID
}

// Kept は、変えずに残したアイテムと、その理由。
type Kept struct {
	Store  port.StoreID
	ID     item.ID
	Reason string
}

// Failed は、保存できなかった Store と、そこで変わるはずだった件数。
type Failed struct {
	Store port.StoreID
	Name  string
	Count int
}

// OffBacklog は、変えたアイテムのうち、完了・中止のためバックログに表示されなくなったものの数。
type OffBacklog struct{ Done, Canceled int }

// Result は、予告と実行の結果。ValueText は、確認の本文の {値}。
type Result struct {
	Changed    int
	Kept       []Kept
	Failed     []Failed
	OffBacklog OffBacklog
	ValueText  string
}

// storeGroup は、1 つの Store で選んだアイテム。
type storeGroup struct {
	id       port.StoreID
	name     string
	readOnly bool
	targets  []Target
}

// sameName は、名前を、前後の空白を除き、大文字と小文字を区別せずに比べる(F2b2-2)。
func sameName(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// groups は、targets を Store ごとに最初に出てきた順で束ねる。同じ (Store, ID) は 2 回目を飛ばす。
// 表示中でない Store を含むときは、何も読み書きせず port.ErrStoreNotFound(F2b2-11)。
func (s Service) groups(ctx context.Context, targets []Target) ([]storeGroup, error) {
	infos, err := s.StoreList.List(ctx)
	if err != nil {
		return nil, err
	}
	var out []storeGroup
	seen := map[Target]bool{}
	for _, t := range targets {
		if seen[t] {
			continue
		}
		seen[t] = true
		i := slices.IndexFunc(infos, func(x port.StoreInfo) bool { return x.ID == t.Store })
		if i < 0 {
			return nil, port.ErrStoreNotFound
		}
		g := slices.IndexFunc(out, func(x storeGroup) bool { return x.id == t.Store })
		if g < 0 {
			out = append(out, storeGroup{id: t.Store, name: infos[i].Name, readOnly: infos[i].ReadOnly})
			g = len(out) - 1
		}
		out[g].targets = append(out[g].targets, t)
	}
	return out, nil
}

// categories は、分類の一覧。
var categories = []workspace.Category{
	workspace.CategoryTodo, workspace.CategoryDoing, workspace.CategoryDone, workspace.CategoryCanceled,
}
