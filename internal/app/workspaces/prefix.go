package workspaces

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ChangePrefix は、接頭辞を変える(03「一般」の「接頭辞」)。番号は変わらず、旧い接頭辞はログに残る。
// 同じ接頭辞なら何もしない(保存しない)。確認ダイアログは画面が出す。
func (s Service) ChangePrefix(ctx context.Context, store port.StoreID, id workspace.ID, prefix string) error {
	prefix = workspace.NormalizePrefix(prefix)
	actor, err := s.me(ctx, store)
	if err != nil {
		return err
	}
	// Summary は Write の前に決めるので、いまの接頭辞は先に読む。書く値は Write の中で読み直して決める。
	current, err := s.Get(ctx, store, id)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: actor, Summary: fmt.Sprintf("接頭辞を %s から %s に変更", current.Workspace.Prefix, prefix)}
	return s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		w, err := tx.Workspaces().Get(id)
		if err != nil {
			return err
		}
		others, err := tx.Workspaces().List()
		if err != nil {
			return err
		}
		next := workspace.Workspace{ID: w.ID, Name: w.Name, Prefix: prefix}
		if err := workspace.CheckPrefix(next, others); err != nil {
			return err
		}
		if prefix == w.Prefix {
			return nil
		}
		return tx.WriteWorkspaces().Append(id, change.NewSet("prefix", w.Prefix, prefix))
	})
}
