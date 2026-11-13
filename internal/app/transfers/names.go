package transfers

import (
	"encoding/json"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// namesOf は、moved_in の names(A5a-2): 写す行 log(create の行は見ない)の f が指す、移動元のワークスペースの
// プロパティ(組み込みは f がキー、追加は fields/<内部 ID>)の内部 ID と名前と、行の from・to の値(配列なら要素)の
// うち、移動元の値(ステータス・優先度・関連システム・各選択肢)の内部 ID と名前。
func namesOf(log []change.Record, from workspace.Settings) map[change.ID]string {
	values := valueNames(from)
	names := map[change.ID]string{}
	for _, r := range log {
		if r.T == change.Create {
			continue
		}
		if p, ok := from.Property(change.ID(strings.TrimPrefix(r.F, item.FieldKey("")))); ok {
			names[p.ID] = p.Name
		}
		for _, id := range append(idsOf(r.From), idsOf(r.To)...) {
			if name, ok := values[id]; ok {
				names[id] = name
			}
		}
	}
	return names
}

// valueNames は、ワークスペースの値(ステータス・優先度・関連システム・各プロパティの選択肢)の内部 ID と名前。
func valueNames(s workspace.Settings) map[change.ID]string {
	lists := [][]workspace.Value{s.Statuses, s.Priorities, s.Systems}
	for _, p := range s.Properties {
		lists = append(lists, p.Choices)
	}
	names := map[change.ID]string{}
	for _, vs := range lists {
		for _, v := range vs {
			names[v.ID] = v.Name
		}
	}
	return names
}

// idsOf は、行の値の JSON が文字列ならそれを、文字列の配列なら要素を返す。ほかの形は空。
func idsOf(raw json.RawMessage) []change.ID {
	var one change.ID
	if json.Unmarshal(raw, &one) == nil {
		return []change.ID{one}
	}
	var many []change.ID
	if json.Unmarshal(raw, &many) == nil {
		return many
	}
	return nil
}
