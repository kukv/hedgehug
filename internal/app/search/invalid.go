package search

import (
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/relation"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// blockedSet は、ブロック中のアイテム(A4a-13)。app/items と同じく、画面に出るアイテムの関連から求め、
// ブロックしているアイテムが閉じたかは、そのアイテムのワークスペースの設定の分類で決める。
func (s *snapshot) blockedSet() map[item.ID]bool {
	return relation.BlockedSet(s.shown, func(it item.Item) bool {
		return s.spaceOf(it.Place.Workspace).settings.Finished(it.Status)
	})
}

// invalidSet は、診断の問題がある、削除されていないアイテム(is:invalid。E1-7)。Store のすべてのアイテム・設定・計画・
// メンバー・Issue のリンクで、repair.ItemProblems を 1 回呼ぶ。
func (s *snapshot) invalidSet() map[item.ID]bool {
	st := repair.StoreState{
		Items: s.items, Settings: map[workspace.ID]workspace.Settings{}, Plans: map[workspace.ID]planning.Plan{},
		Members: s.members, Issues: map[item.ID][]string{},
	}
	for id, sp := range s.spaces {
		if sp.hasSettings {
			st.Settings[id] = sp.settings
		}
		st.Plans[id] = sp.plan
	}
	for _, it := range item.Live(s.items) {
		for _, l := range itemlink.FromLog(s.logs[it.ID]).Links {
			if l.Kind == itemlink.Issue {
				st.Issues[it.ID] = append(st.Issues[it.ID], l.Key())
			}
		}
	}
	out := map[item.ID]bool{}
	for id := range repair.ItemProblems(st) {
		out[id] = true
	}
	return out
}
