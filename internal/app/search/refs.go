package search

import (
	"strings"

	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/query"
	"github.com/kukv/hedgehug/internal/domain/relation"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

// display は、アイテムのいまの ID(HH-12)。
func (s *snapshot) display(it item.Item) string {
	return it.DisplayID(s.spaces[it.Place.Workspace].ws.Prefix)
}

// itemRef は、アイテムのいまの ID と旧 ID(app/palette の索引と同じ求め方。E1-6)。
func (s *snapshot) itemRef(it item.Item, records []transfer.Record) query.Ref {
	display := s.display(it)
	old := s.spaces[it.Place.Workspace].old
	return query.Ref{Display: display, Aliases: aliasIDs(transfer.Aliases(records, transfer.EntityItem, it.ID, old, it.Place.Number, "-"), display)}
}

// projectRef は、プロジェクトのいまの ID と旧 ID。
func (s *snapshot) projectRef(p project.Project, records []transfer.Record) query.Ref {
	sp := s.spaces[p.Place.Workspace]
	if sp == nil {
		return query.Ref{}
	}
	display := p.DisplayID(sp.ws.Prefix)
	return query.Ref{Display: display, Aliases: aliasIDs(transfer.Aliases(records, transfer.EntityProject, p.ID, sp.old, p.Place.Number, "-P"), display)}
}

// aliasIDs は、旧 ID の文字列のうち、いまの ID current と同じものを除いたもの。なければ nil。
func aliasIDs(all []transfer.Alias, current string) []string {
	var out []string
	for _, a := range all {
		if a.ID != current {
			out = append(out, a.ID)
		}
	}
	return out
}

// ancestors は、親と、親から上へたどった祖先(親を含む、近い順)。画面に出るものだけをたどり、
// 画面に出ない親か、一度通ったアイテム(親の循環)で止める。
func (s *snapshot) ancestors(it item.Item, records []transfer.Record) (*query.Ref, []query.Ref) {
	var out []query.Ref
	seen := map[item.ID]bool{it.ID: true}
	for cur := it; cur.Place.Parent != nil; {
		p, ok := s.byID[*cur.Place.Parent]
		if !ok || seen[p.ID] || !s.isShown(p) {
			break
		}
		seen[p.ID] = true
		out = append(out, s.itemRef(p, records))
		cur = p
	}
	if len(out) == 0 {
		return nil, nil
	}
	parent := out[0]
	return &parent, out
}

// relations は、関連のキー(blocks・blocked_by など。表記の空白を _ にしたもの)→ 相手。両方向を入れ、
// 画面に出ない相手(削除済み)は除く。it が画面に出ないもの(削除済み)なら、it 自身が持つ関連も見るため it を足して求める。
func (s *snapshot) relations(it item.Item, records []transfer.Record) map[string][]query.Ref {
	all := s.shown
	if !s.isShown(it) {
		all = append(all[:len(all):len(all)], it)
	}
	var out map[string][]query.Ref
	for _, row := range relation.Rows(it.ID, all) {
		other, ok := s.byID[row.Other]
		if !ok || !s.isShown(other) {
			continue
		}
		if out == nil {
			out = map[string][]query.Ref{}
		}
		key := strings.ReplaceAll(string(row.Label), " ", "_")
		out[key] = append(out[key], s.itemRef(other, records))
	}
	return out
}
