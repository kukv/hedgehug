package search

import (
	"context"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/query"
)

// Row は、検索の結果の 1 行。DaysLeft は削除済みだけで、完全に削除されるまでの日数(0 以下なら保持日数を過ぎている)。
type Row struct {
	Store    port.StoreInfo
	Facts    query.Facts
	OldID    bool
	DaysLeft *int
	Cells    Cells
}

// Result は、検索の結果。Errors があれば Rows は空(アイテムは読まない)。
type Result struct {
	Errors []string
	HasKey bool
	// HasSort は、クエリに sort: の語があるか(エラーのときも解析の結果から入れる)。
	HasSort bool
	Group   string
	Deleted bool
	Rows    []Row
	// Columns は、テーブルの列(クエリのエラーでも返す)。GroupBy は使ったまとめ方(なければ "")、Groups は
	// そのグループ(Rows の位置。E2a-6)。
	Columns []Column
	GroupBy string
	Groups  []query.Group
}

// TableOptions は、テーブルの設定。Sort は列見出しの並べ替え(あれば sort: の代わりに使う)、Group はテーブルの
// 「グループ」の設定(none か空なら設定なし)。
type TableOptions struct {
	Sort  *query.Sort
	Group string
}

// Search は、text を解析し、表示中のすべての Store のアイテムのうち一致するものを、列見出しか sort: の並び(なければ
// Store・ワークスペース・アイテムの並び順)で返す。追加プロパティのキーは、非表示の Store のものも知っているとする。
func (s Service) Search(ctx context.Context, text string, opt TableOptions) (Result, error) {
	keys, columns, err := s.catalog(ctx)
	if err != nil {
		return Result{}, err
	}
	q, errs := query.Parse(text, func(key string) bool { return keys[key] })
	res := Result{Errors: errs, HasKey: q.HasKey, HasSort: q.Sort != nil, Group: q.Group, Deleted: q.Deleted, Columns: columns}
	if len(errs) > 0 {
		return res, nil
	}
	sortBy := q.Sort
	if opt.Sort != nil {
		sortBy = opt.Sort
	}
	rows, now, err := s.collect(ctx, q, res.Columns, sortBy)
	if err != nil {
		return Result{}, err
	}
	res.Rows = make([]Row, 0, len(rows))
	facts := make([]*query.Facts, 0, len(rows))
	for _, r := range rows {
		res.Rows = append(res.Rows, *r)
		facts = append(facts, &r.Facts)
	}
	res.GroupBy = query.GroupKey(opt.Group, q.Group)
	res.Groups = query.Groups(facts, res.GroupBy, now)
	return res, nil
}

// collect は、解析したクエリに一致する行を、sortBy(なければ Store・ワークスペース・アイテムの並び順)で集める。
// 時計は 1 回だけ読み、その時刻も返す。
func (s Service) collect(ctx context.Context, q query.Query, columns []Column, sortBy *query.Sort) ([]*Row, time.Time, error) {
	now := s.Clock.Now()
	es, err := s.entries(ctx, q, now)
	if err != nil {
		return nil, now, err
	}
	today := planning.Today(now)
	var rows []*Row
	for _, e := range es {
		if hit := query.Match(q, e.facts, today); hit.Match {
			rows = append(rows, &Row{Store: e.info, Facts: e.facts, OldID: hit.OldID, DaysLeft: e.daysLeft, Cells: cellsOf(e.facts, columns)})
		}
	}
	query.Order(rows, func(r *Row) *query.Facts { return &r.Facts }, sortBy)
	return rows, now, nil
}
