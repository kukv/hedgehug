package workspaces

import (
	"context"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// RenameValue は、値の名前を変える。同じ名前(完全一致)なら何もしない。
func (s Service) RenameValue(ctx context.Context, store port.StoreID, ws workspace.ID, l workspace.List, id change.ID, name string) error {
	name = workspace.NormalizeName(name)
	return s.changeValue(ctx, store, ws, l, id,
		func(st workspace.Settings, v workspace.Value) string {
			return fmt.Sprintf("%s「%s」を「%s」に改名", st.Label(l), oneLine(v.Name), oneLine(name))
		},
		func(_ port.Tx, _ workspace.Settings, vs []workspace.Value, v workspace.Value) ([]change.Draft, error) {
			if err := workspace.CheckValueName(vs, id, name); err != nil {
				return nil, err
			}
			if name == v.Name {
				return nil, nil
			}
			return []change.Draft{change.NewSet(l.Field(id, "name"), v.Name, name)}, nil
		})
}

// SetValueColor は、値の色を変える。18 色にない色は ErrNotFound、同じ色なら何もしない。
func (s Service) SetValueColor(ctx context.Context, store port.StoreID, ws workspace.ID, l workspace.List, id change.ID, color workspace.Color) error {
	if !slices.Contains(workspace.Colors, color) {
		return port.ErrNotFound
	}
	return s.changeValue(ctx, store, ws, l, id,
		func(st workspace.Settings, v workspace.Value) string {
			return fmt.Sprintf("%s「%s」の色を変更", st.Label(l), oneLine(v.Name))
		},
		func(_ port.Tx, _ workspace.Settings, _ []workspace.Value, v workspace.Value) ([]change.Draft, error) {
			if v.Color == color {
				return nil, nil
			}
			return []change.Draft{change.NewSet(l.Field(id, "color"), v.Color, color)}, nil
		})
}

// MoveValue は、値を 1 つ上(up)または下へ動かす。端なら何もしない。隣と rank を入れ替える。
func (s Service) MoveValue(ctx context.Context, store port.StoreID, ws workspace.ID, l workspace.List, id change.ID, up bool) error {
	return s.changeValue(ctx, store, ws, l, id,
		func(st workspace.Settings, _ workspace.Value) string { return st.Label(l) + "の順番を変更" },
		func(_ port.Tx, _ workspace.Settings, vs []workspace.Value, _ workspace.Value) ([]change.Draft, error) {
			i := slices.IndexFunc(vs, func(v workspace.Value) bool { return v.ID == id })
			j := i + 1
			if up {
				j = i - 1
			}
			if j < 0 || j >= len(vs) {
				return nil, nil
			}
			a, b := vs[i], vs[j]
			return workspace.SwapRanks(l.Field(a.ID, "rank"), a.Rank, l.Field(b.ID, "rank"), b.Rank), nil
		})
}
