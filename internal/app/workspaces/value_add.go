package workspaces

import (
	"context"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// valueOf は、一覧 l の値 id。一覧か値がなければ ErrNotFound。
func valueOf(st workspace.Settings, l workspace.List, id change.ID) ([]workspace.Value, workspace.Value, error) {
	vs, ok := st.Values(l)
	if !ok {
		return nil, workspace.Value{}, port.ErrNotFound
	}
	i := slices.IndexFunc(vs, func(v workspace.Value) bool { return v.ID == id })
	if i < 0 {
		return nil, workspace.Value{}, port.ErrNotFound
	}
	return vs, vs[i], nil
}

// editValue は、一覧 l の値 id に当てる 1 回の変更。ない一覧・値は、検査の前に ErrNotFound。
// text は読んだ設定の値からアクティビティの文を作り、rows は書く Tx の中の設定から行を作る。
func (s Service) editValue(ctx context.Context, store port.StoreID, ws workspace.ID, l workspace.List, id change.ID,
	text func(st workspace.Settings, v workspace.Value) string,
	rows func(tx port.Tx, st workspace.Settings, vs []workspace.Value, v workspace.Value) ([]change.Draft, error),
) error {
	return s.edit(ctx, store, ws,
		func(st workspace.Settings) (string, error) {
			_, v, err := valueOf(st, l, id)
			if err != nil {
				return "", err
			}
			return text(st, v), nil
		},
		func(tx port.Tx, st workspace.Settings) ([]change.Draft, error) {
			vs, v, err := valueOf(st, l, id)
			if err != nil {
				return nil, err
			}
			return rows(tx, st, vs, v)
		})
}

// AddValue は、一覧 l に値を足す。色は並びの順に 18 色を回し、分類は未着手、並びは末尾。
func (s Service) AddValue(ctx context.Context, store port.StoreID, ws workspace.ID, l workspace.List, name string) (workspace.Value, error) {
	v := workspace.Value{ID: s.IDs.New(), Name: workspace.NormalizeName(name)}
	err := s.edit(ctx, store, ws,
		func(st workspace.Settings) (string, error) {
			if _, ok := st.Values(l); !ok {
				return "", port.ErrNotFound
			}
			return fmt.Sprintf("%sに「%s」を追加", st.Label(l), oneLine(v.Name)), nil
		},
		func(_ port.Tx, st workspace.Settings) ([]change.Draft, error) {
			vs, ok := st.Values(l)
			if !ok {
				return nil, port.ErrNotFound
			}
			if err := workspace.CheckValueName(vs, "", v.Name); err != nil {
				return nil, err
			}
			ranks := make([]string, 0, len(vs))
			for _, x := range vs {
				ranks = append(ranks, x.Rank)
			}
			v.Color = workspace.Colors[len(vs)%len(workspace.Colors)]
			v.Rank = workspace.NextRank(ranks...)
			if l.Kind == workspace.ListStatuses {
				v.Category = workspace.CategoryTodo
			}
			return workspace.ValueDrafts(l, v), nil
		})
	if err != nil {
		return workspace.Value{}, err
	}
	return v, nil
}
