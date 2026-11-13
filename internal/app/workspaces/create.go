package workspaces

import (
	"context"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// oneLine は、アクティビティの 1 行にするため、改行を含む連続する空白を 1 つの空白にまとめる。
// 保存する名前は変えない(03「名前の重複」)。
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// Create は、store にワークスペースを作る(03「ワークスペースの作成」)。初期値は DefaultSettings。
// 検査は 03 の表の順に行い、最初に当たったものの文言で断る。
func (s Service) Create(ctx context.Context, store port.StoreID, name, prefix string) (workspace.Workspace, error) {
	name, prefix = workspace.NormalizeName(name), workspace.NormalizePrefix(prefix)
	actor, err := s.me(ctx, store)
	if err != nil {
		return workspace.Workspace{}, err
	}
	w := workspace.Workspace{ID: s.IDs.New(), Name: name, Prefix: prefix}
	op := port.Operation{Actor: actor, Summary: fmt.Sprintf("ワークスペース「%s」を作成", oneLine(name))}
	err = s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		others, err := port.LiveWorkspaces(tx)
		if err != nil {
			return err
		}
		if err := workspace.CheckCreate(w, others); err != nil {
			return err
		}
		ordered, err := orderedWorkspaces(tx) // 削除済みも含める(rank を使い回さない)
		if err != nil {
			return err
		}
		ranks := make([]string, len(ordered))
		for i, o := range ordered {
			ranks[i] = o.rank
		}
		draft, err := workspace.CreateWithRank(w, workspace.DefaultSettings(s.IDs.New), workspace.NextRank(ranks...))
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
