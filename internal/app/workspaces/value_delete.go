package workspaces

import (
	"context"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// DeleteValue は、値を消す(03「削除」)。使っているアイテムは、代わりの値に置き換わるか値が外れる。
// replacement が空なら、ステータス・優先度は初期値、単一選択は「なし」。
func (s Service) DeleteValue(ctx context.Context, store port.StoreID, ws workspace.ID, l workspace.List, id, replacement change.ID) error {
	return s.editValue(ctx, store, ws, l, id,
		func(st workspace.Settings, v workspace.Value) string {
			return fmt.Sprintf("%s「%s」を削除", st.Label(l), oneLine(v.Name))
		},
		func(tx port.Tx, st workspace.Settings, vs []workspace.Value, _ workspace.Value) ([]change.Draft, error) {
			repl, err := deleteChecked(st, l, vs, id, replacement)
			if err != nil {
				return nil, err
			}
			its, err := workspaceItems(tx, ws)
			if err != nil {
				return nil, err
			}
			for _, it := range its {
				if ds := replaceRows(st, l, it, id, repl); len(ds) > 0 {
					if err := tx.WriteItems().Append(it.ID, ds...); err != nil {
						return nil, err
					}
				}
			}
			return []change.Draft{change.NewDelete(l.Ref(id))}, nil
		})
}

// deleteChecked は、削除の検査をして、置き換える値を決める(B2-12)。
func deleteChecked(st workspace.Settings, l workspace.List, vs []workspace.Value, id, repl change.ID) (change.ID, error) {
	switch l.Kind {
	case workspace.ListStatuses:
		if err := workspace.CheckDeleteStatus(st, id); err != nil {
			return "", err
		}
	case workspace.ListPriorities:
		if err := workspace.CheckDeletePriority(st, id); err != nil {
			return "", err
		}
	}
	if repl == "" {
		return initialOf(st, l), nil
	}
	if !st.Single(l) || repl == id || !slices.ContainsFunc(vs, func(v workspace.Value) bool { return v.ID == repl }) {
		return "", port.ErrNotFound
	}
	return repl, nil
}

// initialOf は、代わりの値を渡さないときの置き換え先。ステータスと優先度は初期値、ほかは空。
func initialOf(st workspace.Settings, l workspace.List) change.ID {
	switch l.Kind {
	case workspace.ListStatuses:
		return st.InitialStatus
	case workspace.ListPriorities:
		return st.InitialPriority
	}
	return ""
}

// replaceRows は、アイテム it から値 id を外す・置き換える行(B2-8)。
func replaceRows(st workspace.Settings, l workspace.List, it item.Item, id, repl change.ID) []change.Draft {
	switch l.Kind {
	case workspace.ListStatuses:
		if it.Status == id {
			return []change.Draft{change.NewSet("status", id, repl)}
		}
	case workspace.ListPriorities:
		if it.Priority == id {
			return []change.Draft{change.NewSet("priority", id, repl)}
		}
	case workspace.ListSystems:
		if slices.Contains(it.Systems, id) {
			return []change.Draft{change.NewRemove("systems", id)}
		}
	case workspace.ListChoices:
		return replaceChoice(st, l, it.Choices[l.Property], id, repl)
	}
	return nil
}

func replaceChoice(st workspace.Settings, l workspace.List, vs []change.ID, id, repl change.ID) []change.Draft {
	if !slices.Contains(vs, id) {
		return nil
	}
	f := item.FieldKey(l.Property)
	switch {
	case !st.Single(l):
		return []change.Draft{change.NewRemove(f, id)}
	case repl == "":
		return []change.Draft{change.NewSet(f, id, nil)}
	}
	return []change.Draft{change.NewSet(f, id, repl)}
}
