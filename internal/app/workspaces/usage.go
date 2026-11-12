package workspaces

import (
	"context"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Usage は、ワークスペースのアイテムの使われ方(B2-11)。
type Usage struct {
	Items        int               // アイテムの数(接頭辞の確認の N)
	FirstNumbers []int             // 番号の小さい方から 3 つ(接頭辞の ID の例)
	Values       map[change.ID]int // 値の内部 ID ごとの、その値を持つアイテムの数
	Properties   map[change.ID]int // 追加プロパティごとの、値の入っているアイテムの数
}

// Usage は、ws のアイテムの件数と、値・プロパティごとの件数を返す。ワークスペースがなければ ErrNotFound。
func (s Service) Usage(ctx context.Context, store port.StoreID, ws workspace.ID) (Usage, error) {
	u := Usage{Values: map[change.ID]int{}, Properties: map[change.ID]int{}}
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		if _, err := r.Workspaces().Get(ws); err != nil {
			return err
		}
		items, err := workspaceItems(r, ws)
		if err != nil {
			return err
		}
		var numbers []int
		for _, it := range items {
			numbers = append(numbers, it.Place.Number)
			u.count(it)
		}
		slices.Sort(numbers)
		u.Items, u.FirstNumbers = len(numbers), numbers[:min(3, len(numbers))]
		return nil
	})
	return u, err
}

// count は、アイテム 1 つが持つ値とプロパティを数える。
func (u Usage) count(it item.Item) {
	for _, v := range append([]change.ID{it.Status, it.Priority}, it.Systems...) {
		if v != "" {
			u.Values[v]++
		}
	}
	for p, vs := range it.Choices {
		if len(vs) > 0 {
			u.Properties[p]++
		}
		for _, v := range vs {
			u.Values[v]++
		}
	}
}
