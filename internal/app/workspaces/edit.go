package workspaces

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// edit は、設定の 1 回の変更(01「書き込みの単位」)。アクティビティの文は保存の前に決めるので、読んだ設定から
// summary で作る。行は、書く Tx の中で読み直した設定から rows で作る(検査もここで行う)。行がなければ何も
// 保存しない。rows はアイテムにも書いてよい(同じ保存に入る)。
func (s Service) edit(ctx context.Context, store port.StoreID, ws workspace.ID,
	summary func(workspace.Settings) (string, error),
	rows func(tx port.Tx, st workspace.Settings) ([]change.Draft, error),
) error {
	actor, err := s.me(ctx, store)
	if err != nil {
		return err
	}
	var st workspace.Settings
	if err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		if _, err := port.LiveWorkspace(r, ws); err != nil {
			return err
		}
		var err error
		st, err = r.Workspaces().Settings(ws)
		return err
	}); err != nil {
		return err
	}
	text, err := summary(st)
	if err != nil {
		return err
	}
	return s.Stores.Write(ctx, store, port.Operation{Actor: actor, Summary: text}, func(tx port.Tx) error {
		if _, err := port.LiveWorkspace(tx, ws); err != nil {
			return err
		}
		st, err := tx.Workspaces().Settings(ws)
		if err != nil {
			return err
		}
		ds, err := rows(tx, st)
		if err != nil {
			return err
		}
		return tx.WriteWorkspaces().Append(ws, ds...)
	})
}
