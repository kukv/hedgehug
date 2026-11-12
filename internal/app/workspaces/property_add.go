package workspaces

import (
	"context"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// NewProperty は、追加プロパティのダイアログの入力(03「追加プロパティ」)。
type NewProperty struct {
	Name  string
	Key   string
	Type  workspace.PropertyType
	Kinds []string // 空ならすべて
}

// AddProperty は、追加プロパティを作る。
func (s Service) AddProperty(ctx context.Context, store port.StoreID, ws workspace.ID, in NewProperty) (workspace.Property, error) {
	if !slices.Contains(workspace.AddableTypes, in.Type) {
		return workspace.Property{}, port.ErrNotFound
	}
	p := workspace.Property{
		ID: s.IDs.New(), Key: in.Key, Name: workspace.NormalizeName(in.Name), Type: in.Type,
		Kinds: workspace.NormalizeKinds(in.Kinds),
	}
	if len(p.Kinds) == 0 {
		p.Kinds = slices.Clone(workspace.Kinds)
	}
	err := s.edit(ctx, store, ws,
		func(workspace.Settings) (string, error) {
			return fmt.Sprintf("プロパティ「%s」を追加", oneLine(p.Name)), nil
		},
		func(_ port.Tx, st workspace.Settings) ([]change.Draft, error) {
			if err := workspace.CheckNewProperty(st, p.Name, p.Key); err != nil {
				return nil, err
			}
			ranks := make([]string, 0, len(st.Properties))
			for _, x := range st.Properties {
				ranks = append(ranks, x.Rank)
			}
			p.Rank = workspace.NextRank(ranks...)
			return workspace.PropertyDrafts(p), nil
		})
	if err != nil {
		return workspace.Property{}, err
	}
	return p, nil
}
