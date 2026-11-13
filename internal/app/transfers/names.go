package transfers

import (
	"encoding/json"
	"maps"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// namesOf は、moved_in の names(A5a-2): 写す行 log の f が指すプロパティ(組み込みは f がキー、追加は fields/<内部 ID>)の
// 内部 ID と名前と、行の from・to の値(配列なら要素)の内部 ID と名前。名前は設定 vocab(移動元のワークスペースが先頭、
// 続いて同じ Store の中で前にいたワークスペース)の、先にあるものから引く。作成の行は、to の各キーを f、その値を値として
// 同じに扱う(作成のときにだけ設定した値も、移動の引き直しの行の from に現れるため)。
func namesOf(log []change.Record, vocab []workspace.Settings) map[change.ID]string {
	n := nameSet{vocab: vocab, values: map[change.ID]string{}, names: map[change.ID]string{}}
	for i := len(vocab) - 1; i >= 0; i-- { // 先のものが勝つ
		maps.Copy(n.values, valueNames(vocab[i]))
	}
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
	vocab  []workspace.Settings
	values map[change.ID]string
	names  map[change.ID]string
}

// add は、項目名 f が指すプロパティと、raws に現れる値の名前を足す。
func (n nameSet) add(f string, raws ...json.RawMessage) {
	id := change.ID(strings.TrimPrefix(f, item.FieldKey("")))
	for _, st := range n.vocab {
		if p, ok := st.Property(id); ok {
			n.names[p.ID] = p.Name
			break
		}
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

// placesOf は、場所の set の行の from と to。場所の set でなければ ok=false。
func placesOf(r change.Record) (from, to item.Place, ok bool) {
	if r.T != change.Set || r.F != "place" || json.Unmarshal(r.From, &from) != nil || json.Unmarshal(r.To, &to) != nil {
		return from, to, false
	}
	return from, to, true
}

// workspacesIn は、行 log の作成の場所と、場所の set の from・to に現れるワークスペース(現れた順。重なりあり)。
func workspacesIn(log []change.Record) []workspace.ID {
	var out []workspace.ID
	for _, r := range log {
		if r.T == change.Create {
			var v struct{ Place item.Place }
			if json.Unmarshal(r.To, &v) == nil && v.Place.Workspace != "" {
				out = append(out, v.Place.Workspace)
			}
		}
		if from, to, ok := placesOf(r); ok {
			out = append(out, from.Workspace, to.Workspace)
		}
	}
	return out
}
