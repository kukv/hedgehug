package search

import (
	"context"

	"github.com/kukv/hedgehug/internal/domain/query"
)

// MyTasksQuery は、マイタスクの基本のクエリ(04「マイタスク」)。ユーザーの絞り込みはこの後ろに AND でつなぐ。
const MyTasksQuery = "assignee:@me -category:done,canceled -kind:epic"

// MyTasks は、基本のクエリにユーザーの絞り込み filter をつないで検索する。sort:・group: は無視し、
// Store・ワークスペース・アイテムの並び順で返す。Result は Errors・Rows だけを使う。
func (s Service) MyTasks(ctx context.Context, filter string) (Result, error) {
	keys, _, err := s.catalog(ctx)
	if err != nil {
		return Result{}, err
	}
	q, errs := query.Parse(MyTasksQuery+" "+filter, func(key string) bool { return keys[key] })
	if len(errs) > 0 {
		return Result{Errors: errs}, nil
	}
	q.Sort, q.Group = nil, ""
	rows, _, err := s.collect(ctx, q, nil, nil)
	if err != nil {
		return Result{}, err
	}
	res := Result{Rows: make([]Row, 0, len(rows))}
	for _, r := range rows {
		res.Rows = append(res.Rows, *r)
	}
	return res, nil
}
