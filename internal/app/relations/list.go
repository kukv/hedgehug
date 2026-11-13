package relations

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/relation"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Row は、関連の一覧の 1 行。
type Row struct {
	Label         relation.Label
	Item          item.Item // 相手
	Display       string    // 相手の画面の ID
	WorkspaceName string    // 相手が別のワークスペースのときだけ
	Status        workspace.Value
	HasStatus     bool
}

// List は、self の関連を、一覧の順(表記の順)で返す。逆向きの表記を含む。self がなければ port.ErrNotFound。
func (s Service) List(ctx context.Context, store port.StoreID, self item.ID) ([]Row, error) {
	var out []Row
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		snap, err := readSnapshot(r)
		if err != nil {
			return err
		}
		me, ok := snap.byID[self]
		if !ok {
			return port.ErrNotFound
		}
		for _, row := range relation.Rows(self, snap.all) {
			other := snap.byID[row.Other]
			v := Row{Label: row.Label, Item: other, Display: snap.display(other), WorkspaceName: snap.workspaceNameFor(me, other)}
			v.Status, v.HasStatus = snap.status(other)
			out = append(out, v)
		}
		return nil
	})
	return out, err
}
