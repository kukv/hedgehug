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
	return s.remove(ctx, store, ws,
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
			ds := []change.Draft{change.NewDelete("properties/" + string(id))}
			return append(ds, workspace.TemplatePropertyDrafts(st, id)...), nil
		})
}

// clearItems は、ワークスペースのアイテムからプロパティ p の値をすべて外す(同じ保存に入る)。
func clearItems(tx port.Tx, ws workspace.ID, p workspace.Property) error {
	its, err := workspaceItems(tx, ws)
	if err != nil {
		return err
	}
	for _, it := range its {
		if ds := clearValue(p, it); len(ds) > 0 {
			if err := tx.WriteItems().Append(it.ID, ds...); err != nil {
				return err
			}
		}
	}
	return nil
}

// clearValue は、アイテム it からプロパティ p の値を外す行(型で読み分ける、A3 前半の決定 9・32)。
// 複数選択は値ごとに remove、ほかの型は値が入っていれば set null。
func clearValue(p workspace.Property, it item.Item) []change.Draft {
	f := item.FieldKey(p.ID)
	if p.Type != workspace.TypeMulti {
		if raw, ok := it.Fields[p.ID]; ok {
			return []change.Draft{change.NewSet(f, raw, nil)}
		}
		return nil
	}
	vs := it.Choices[p.ID]
	ds := make([]change.Draft, 0, len(vs))
	for _, v := range vs {
		ds = append(ds, change.NewRemove(f, v))
	}
	return ds
}
