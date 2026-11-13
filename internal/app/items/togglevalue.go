package items

import (
	"context"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// AddValue は、関連システムか複数選択に値を 1 つ足す(02「入力の方法と保存」)。すでにあれば何も保存しない。
func (s Service) AddValue(ctx context.Context, store port.StoreID, id item.ID, prop, value change.ID) (item.Item, error) {
	return s.toggle(ctx, store, id, prop, value, true)
}

// RemoveValue は、値を 1 つ取り除く。なければ何も保存しない。
func (s Service) RemoveValue(ctx context.Context, store port.StoreID, id item.ID, prop, value change.ID) (item.Item, error) {
	return s.toggle(ctx, store, id, prop, value, false)
}

func (s Service) toggle(ctx context.Context, store port.StoreID, id item.ID, prop, value change.ID, on bool) (item.Item, error) {
	ec, err := s.editContext(ctx, store, id)
	if err != nil {
		return item.Item{}, err
	}
	p, err := ec.property(prop)
	if err != nil {
		return item.Item{}, err
	}
	list, ok := listOf(ec.settings, p)
	if !ok || (on && !slices.ContainsFunc(list, func(v workspace.Value) bool { return v.ID == value })) {
		return item.Item{}, port.ErrNotFound
	}
	after := toggled(currentIDs(ec.item, p), value, on)
	summary := fmt.Sprintf("%s の%sを %s に変更", ec.display, p.Name, item.Display(p, change.Value(after), ec.settings, ec.names, s.Clock.Now()))
	return s.writeValue(ctx, store, id, summary, func(_ port.Tx, it item.Item) ([]change.Draft, error) {
		has := slices.Contains(currentIDs(it, p), value)
		switch {
		case on && !has:
			return []change.Draft{change.NewAdd(item.Field(p), value)}, nil
		case !on && has:
			return []change.Draft{change.NewRemove(item.Field(p), value)}, nil
		}
		return nil, nil
	}, nil)
}

func listOf(st workspace.Settings, p workspace.Property) ([]workspace.Value, bool) {
	switch p.Type {
	case workspace.TypeSystems:
		return st.Systems, true
	case workspace.TypeMulti:
		return p.Choices, true
	}
	return nil, false
}

func currentIDs(it item.Item, p workspace.Property) []change.ID {
	if p.Type == workspace.TypeSystems {
		return it.Systems
	}
	return it.Choices[p.ID]
}

func toggled(ids []change.ID, v change.ID, on bool) []change.ID {
	out := slices.DeleteFunc(slices.Clone(ids), func(x change.ID) bool { return x == v })
	if on {
		out = append(out, v)
	}
	return out
}
