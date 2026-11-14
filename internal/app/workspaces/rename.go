package workspaces

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Rename は、ワークスペースの名前を変える(03「一般」の「名前」)。同じ名前なら何もしない(保存しない)。
// 削除済みのワークスペースなら、変更後の名前で判定して削除を取り消す。同じ名前なら ErrNotFound(決定 R11)。
func (s Service) Rename(ctx context.Context, store port.StoreID, id workspace.ID, name string) error {
	name = workspace.NormalizeName(name)
	actor, err := s.me(ctx, store)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: actor, Summary: fmt.Sprintf("ワークスペースの名前を「%s」に変更", oneLine(name))}
	return s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		// 入力の検査で断るものは、取り消しの判定(08 の断り)より先に断る
		if err := workspace.CheckRename(workspace.Workspace{ID: id, Name: name}, nil); err != nil {
			return err
		}
		revived, err := port.ReviveWorkspace(ctx, tx, id, name, "")
		if err != nil {
			return err
		}
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
			return unchanged(revived)
		}
		return tx.WriteWorkspaces().Append(id, change.NewSet("name", w.Name, name))
	})
}

// unchanged は、行を書かない保存の結果。ワークスペースの削除を取り消した(revived)なら ErrNotFound(決定 R11)。
func unchanged(revived bool) error {
	if revived {
		return port.ErrNotFound
	}
	return nil
}
