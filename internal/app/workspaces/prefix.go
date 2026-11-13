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
	// Operation は Write の前に渡す口の形なので、Summary の旧い接頭辞は先に読んだ値から作る。
	// 読んでから書くまでに別の変更が入ると、文だけがずれうる。書く値は Write の中で読み直して決める。
	var current workspace.Workspace
	if err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		current, err = port.LiveWorkspace(r, id)
		return err
	}); err != nil {
		return err
	}
	op := port.Operation{Actor: actor, Summary: fmt.Sprintf("接頭辞を %s から %s に変更", current.Prefix, prefix)}
	return s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		w, err := port.LiveWorkspace(tx, id)
		if err != nil {
			return err
		}
		others, err := port.LiveWorkspaces(tx)
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
