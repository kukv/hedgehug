package workspaces

import (
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// workspaceItems は、ワークスペースのすべてのアイテム(プロジェクトの並び、番号の順)。削除済みのアイテム・
// プロジェクトも含む。値の置き換えと、プロパティの値を消すのは、削除済みも対象にする(03「削除」。A4b-5)。
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

// liveWorkspaceItems は、workspaceItems から、削除済みのアイテムと、削除済みのプロジェクトのアイテムを除いたもの。
// 件数・番号の例のように、画面に見えるものだけを数えるときに使う(B2-11・A4b-5)。
func liveWorkspaceItems(r port.Reader, ws workspace.ID) ([]item.Item, error) {
	ps, err := port.LiveProjects(r, ws)
	if err != nil {
		return nil, err
	}
	all, err := workspaceItems(r, ws)
	if err != nil {
		return nil, err
	}
	live := map[project.ID]bool{}
	for _, p := range ps {
		live[p.ID] = true
	}
	return slices.DeleteFunc(item.Live(all), func(it item.Item) bool { return !live[it.Place.Project] }), nil
}
