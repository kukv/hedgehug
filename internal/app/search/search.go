package search

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/query"
)

// Row は、検索の結果の 1 行。DaysLeft は削除済みだけで、完全に削除されるまでの日数(保持日数を過ぎていれば負)。
type Row struct {
	Store    port.StoreInfo
	Facts    query.Facts
	OldID    bool
	DaysLeft *int
}

// Result は、検索の結果。Errors があれば Rows は空(アイテムは読まない)。
type Result struct {
	Errors  []string
	HasKey  bool
	Group   string
	Deleted bool
	Rows    []Row
}

// Search は、text を解析し、表示中のすべての Store のアイテムのうち一致するものを、sort: の並び(なければ
// Store・ワークスペース・アイテムの並び順)で返す。追加プロパティのキーは、非表示の Store のものも知っているとする。
func (s Service) Search(ctx context.Context, text string) (Result, error) {
	keys, err := s.customKeys(ctx)
	if err != nil {
		return Result{}, err
	}
	q, errs := query.Parse(text, func(key string) bool { return keys[key] })
	res := Result{Errors: errs, HasKey: q.HasKey, Group: q.Group, Deleted: q.Deleted}
	if len(errs) > 0 {
		return res, nil
	}
	es, err := s.entries(ctx, q)
	if err != nil {
		return Result{}, err
	}
	today := planning.Today(s.Clock.Now())
	var rows []*Row
	for _, e := range es {
		if hit := query.Match(q, e.facts, today); hit.Match {
			rows = append(rows, &Row{Store: e.info, Facts: e.facts, OldID: hit.OldID, DaysLeft: e.daysLeft})
		}
	}
	query.Order(rows, func(r *Row) *query.Facts { return &r.Facts }, q.Sort)
	res.Rows = make([]Row, 0, len(rows))
	for _, r := range rows {
		res.Rows = append(res.Rows, *r)
	}
	return res, nil
}
