package workspaces

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// DeleteProperty は、追加プロパティを消す(03「プロパティ」の「削除」)。値の入っているアイテムから値を外す。
func (s Service) DeleteProperty(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID) error {
	return s.edit(ctx, store, ws,
		func(st workspace.Settings) (string, error) {
			p, ok := st.Property(id)
			if !ok || p.Builtin {
				return "", port.ErrNotFound
			}
			return fmt.Sprintf("プロパティ「%s」を削除", oneLine(p.Name)), nil
		},
		func(tx port.Tx, st workspace.Settings) ([]change.Draft, error) {
			p, ok := st.Property(id)
			if !ok || p.Builtin {
				return nil, port.ErrNotFound
			}
			if err := clearItems(tx, ws, p); err != nil {
				return nil, err
			}
			return []change.Draft{change.NewDelete("properties/" + string(id))}, nil
		})
}

// clearItems は、ワークスペースのアイテムからプロパティ p の値をすべて外す(同じ保存に入る)。
func clearItems(tx port.Tx, ws workspace.ID, p workspace.Property) error {
	its, err := workspaceItems(tx, ws)
	if err != nil {
		return err
	}
	for _, it := range its {
		if ds := clearChoice(p, it.Choices[p.ID]); len(ds) > 0 {
			if err := tx.WriteItems().Append(it.ID, ds...); err != nil {
				return err
			}
		}
	}
	return nil
}

// clearChoice は、アイテムからプロパティ p の値 vs を外す行。複数選択は値ごとに remove、ほかは set null。
func clearChoice(p workspace.Property, vs []change.ID) []change.Draft {
	f := item.FieldKey(p.ID)
	if len(vs) == 0 {
		return nil
	}
	if p.Type != workspace.TypeMulti {
		return []change.Draft{change.NewSet(f, vs[0], nil)}
	}
	ds := make([]change.Draft, 0, len(vs))
	for _, v := range vs {
		ds = append(ds, change.NewRemove(f, v))
	}
	return ds
}
