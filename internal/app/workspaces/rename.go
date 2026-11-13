package workspaces

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Rename は、ワークスペースの名前を変える(03「一般」の「名前」)。同じ名前なら何もしない(保存しない)。
func (s Service) Rename(ctx context.Context, store port.StoreID, id workspace.ID, name string) error {
	name = workspace.NormalizeName(name)
	actor, err := s.me(ctx, store)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: actor, Summary: fmt.Sprintf("ワークスペースの名前を「%s」に変更", oneLine(name))}
	return s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		w, err := port.LiveWorkspace(tx, id)
		if err != nil {
			return err
		}
		others, err := port.LiveWorkspaces(tx)
		if err != nil {
			return err
		}
		next := workspace.Workspace{ID: w.ID, Name: name, Prefix: w.Prefix}
		if err := workspace.CheckRename(next, others); err != nil {
			return err
		}
		if name == w.Name {
			return nil
		}
		return tx.WriteWorkspaces().Append(id, change.NewSet("name", w.Name, name))
	})
}
