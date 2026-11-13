package transfers

import (
	"encoding/json"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// namesOf は、moved_in の names(A5a-2): 写す行 log の f が指す、移動元のワークスペースのプロパティ(組み込みは
// f がキー、追加は fields/<内部 ID>)の内部 ID と名前と、行の from・to の値(配列なら要素)のうち、移動元の値
// (ステータス・優先度・関連システム・各選択肢)の内部 ID と名前。作成の行は、to の各キーを f、その値を値として同じに扱う
// (作成のときにだけ設定した値も、移動の引き直しの行の from に現れるため)。
func namesOf(log []change.Record, from workspace.Settings) map[change.ID]string {
	n := nameSet{from: from, values: valueNames(from), names: map[change.ID]string{}}
	for _, r := range log {
		if r.T != change.Create {
			n.add(r.F, r.From, r.To)
			continue
		}
		var to map[string]json.RawMessage
		_ = json.Unmarshal(r.To, &to)
		for k, v := range to {
			n.add(k, v)
		}
	}
	return n.names
}

// nameSet は、namesOf が集める names。
type nameSet struct {
	from   workspace.Settings
	values map[change.ID]string
	names  map[change.ID]string
}

// add は、項目名 f が指すプロパティと、raws に現れる移動元の値の名前を足す。
func (n nameSet) add(f string, raws ...json.RawMessage) {
	if p, ok := n.from.Property(change.ID(strings.TrimPrefix(f, item.FieldKey("")))); ok {
		n.names[p.ID] = p.Name
	}
	for _, raw := range raws {
		for _, id := range idsOf(raw) {
			if name, ok := n.values[id]; ok {
				n.names[id] = name
			}
		}
	}
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
