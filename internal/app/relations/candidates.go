package relations

import (
	"context"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/relation"
)

// maxCandidates は、候補の件数の上限(A4a-12。仕様にないので、画面に収まる数)。
const maxCandidates = 20

// Candidate は、相手の候補。DisabledReason が空でなければ選べない。
type Candidate struct {
	Item           item.Item
	Display        string
	WorkspaceName  string // 別のワークスペースの候補にだけ
	DisabledReason string
}

// Candidates は、self が label の関連を足せる相手の候補を返す(02「関連」の「追加」)。自分と Step を除き、
// ID(前方一致)かタイトル(部分一致)で絞る。どちらも大文字小文字を区別しない。ワークスペースの並び → 番号の順で
// 20 件まで。検索語が空なら返さない。知らない表記か self がなければ port.ErrNotFound。
func (s Service) Candidates(ctx context.Context, store port.StoreID, self item.ID, label relation.Label, query string) ([]Candidate, error) {
	typ, reverse, ok := relation.Parse(label)
	if !ok {
		return nil, port.ErrNotFound
	}
	q := strings.ToLower(strings.TrimSpace(query))
	var out []Candidate
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		snap, err := readSnapshot(r)
		if err != nil {
			return err
		}
		me, ok := snap.byID[self]
		if !ok {
			return port.ErrNotFound
		}
		if q == "" {
			return nil
		}
		its := slices.Clone(snap.all)
		slices.SortStableFunc(its, func(a, b item.Item) int {
			if c := slices.Index(snap.order, a.Place.Workspace) - slices.Index(snap.order, b.Place.Workspace); c != 0 {
				return c
			}
			return a.Place.Number - b.Place.Number
		})
		for _, it := range its {
			display := snap.display(it)
			matches := strings.HasPrefix(strings.ToLower(display), q) || strings.Contains(strings.ToLower(it.Title), q)
			if it.ID == self || it.Kind == item.Step || !matches {
				continue
			}
			out = append(out, Candidate{
				Item: it, Display: display, WorkspaceName: snap.workspaceNameFor(me, it),
				DisabledReason: snap.reason(self, it.ID, label, typ, reverse),
			})
			if len(out) == maxCandidates {
				break
			}
		}
		return nil
	})
	return out, err
}
