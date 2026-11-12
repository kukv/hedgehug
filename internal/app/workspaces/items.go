package workspaces

import (
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// workspaceItems は、ワークスペースのすべてのアイテム(プロジェクトの並び、番号の順)。
// アイテムの削除(A4)が入ったら、削除済みを含むかを呼ぶ側で決める(B2-11)。
func workspaceItems(r port.Reader, ws workspace.ID) ([]item.Item, error) {
	ps, err := r.Projects().List(ws)
	if err != nil {
		return nil, err
	}
	var out []item.Item
	for _, p := range ps {
		its, err := r.Items().List(p.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, its...)
	}
	return out, nil
}
