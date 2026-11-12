package workspaces

import (
	"context"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// SetTransition は、遷移の制約の表のセルのチェックを付ける・外す(03「遷移の制約」。チェックのたびに保存)。
// 既存のアイテムは変えない(03「変更が及ぶ範囲」)。
func (s Service) SetTransition(ctx context.Context, store port.StoreID, ws workspace.ID, from, to change.ID, allowed bool) error {
	return s.edit(ctx, store, ws,
		func(st workspace.Settings) (string, error) {
			if err := checkPair(st, from, to); err != nil {
				return "", err
			}
			return "ワークフローの遷移の制約を変更", nil
		},
		func(_ port.Tx, st workspace.Settings) ([]change.Draft, error) {
			if err := checkPair(st, from, to); err != nil {
				return nil, err
			}
			has := slices.Contains(st.Workflow[from], to)
			switch {
			case allowed && !has:
				return []change.Draft{change.NewAdd(workspace.WorkflowField(from), to)}, nil
			case !allowed && has:
				return []change.Draft{change.NewRemove(workspace.WorkflowField(from), to)}, nil
			}
			return nil, nil
		})
}

// checkPair は、表のセルになる組か(どちらもあるステータスで、同じでない)。
func checkPair(st workspace.Settings, from, to change.ID) error {
	has := func(id change.ID) bool {
		return slices.ContainsFunc(st.Statuses, func(v workspace.Value) bool { return v.ID == id })
	}
	if from == to || !has(from) || !has(to) {
		return port.ErrNotFound
	}
	return nil
}
