package workspaces

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Create は、store にワークスペースを作る(03「ワークスペースの作成」)。初期値は DefaultSettings。
// 検査は 03 の表の順に行い、最初に当たったものの文言で断る。
func (s Service) Create(ctx context.Context, store port.StoreID, name, prefix string) (workspace.Workspace, error) {
	name, prefix = workspace.NormalizeName(name), workspace.NormalizePrefix(prefix)
	actor, err := s.me(ctx, store)
	if err != nil {
		return workspace.Workspace{}, err
	}
	w := workspace.Workspace{ID: s.IDs.New(), Name: name, Prefix: prefix}
	op := port.Operation{Actor: actor, Summary: fmt.Sprintf("ワークスペース「%s」を作成", name)}
	err = s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		others, err := tx.Workspaces().List()
		if err != nil {
			return err
		}
		if err := workspace.CheckCreate(w, others); err != nil {
			return err
		}
		draft, err := workspace.CreateWithSettings(w, workspace.DefaultSettings(s.IDs.New))
		if err != nil {
			return err
		}
		return tx.WriteWorkspaces().Append(w.ID, draft)
	})
	if err != nil {
		return workspace.Workspace{}, err
	}
	return w, nil
}
