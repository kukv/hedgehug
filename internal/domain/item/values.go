package item

import (
	"encoding/json"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
)

const fieldsPrefix = "fields/"

// FieldKey は、追加プロパティの値の項目名(保存形式「変更記録」の fields/<プロパティの内部 ID>)。
func FieldKey(property change.ID) string { return fieldsPrefix + string(property) }

// MultiChoice は、追加プロパティ p の値を複数選択の形(add・remove か、作成の配列)で持っているか。
func MultiChoice(log []change.Record, p change.ID) bool { return multi(log)(FieldKey(p)) }

// multi は、値を複数持つ項目か。fields/<ID> は、その項目に add・remove の行があるか、作成の値が
// 配列なら複数選択(B2-2)。domain/item はプロパティの型を知らないので、行の形で決める。
func multi(log []change.Record) func(f string) bool {
	lists := map[string]bool{"watchers": true, "systems": true}
	for _, r := range log {
		switch {
		case (r.T == change.Add || r.T == change.Remove) && strings.HasPrefix(r.F, fieldsPrefix):
			lists[r.F] = true
		case r.T == change.Create:
			var to map[string]json.RawMessage
			_ = json.Unmarshal(r.To, &to)
			for f, v := range to {
				if strings.HasPrefix(f, fieldsPrefix) && strings.HasPrefix(strings.TrimSpace(string(v)), "[") {
					lists[f] = true
				}
			}
		}
	}
	return func(f string) bool { return lists[f] }
}

func readValues(it *Item, s change.State) {
	it.Status = id(s.Values["status"])
	it.Priority = id(s.Values["priority"])
	it.Systems = ids(s.Lists["systems"])
	for f, raw := range s.Values {
		if v := id(raw); strings.HasPrefix(f, fieldsPrefix) && v != "" {
			it.choice(change.ID(strings.TrimPrefix(f, fieldsPrefix)), []change.ID{v})
		}
	}
	for f, list := range s.Lists {
		if vs := ids(list); strings.HasPrefix(f, fieldsPrefix) && len(vs) > 0 {
			it.choice(change.ID(strings.TrimPrefix(f, fieldsPrefix)), vs)
		}
	}
}

func (it *Item) choice(p change.ID, vs []change.ID) {
	if it.Choices == nil {
		it.Choices = map[change.ID][]change.ID{}
	}
	it.Choices[p] = vs
}

// id は、値の内部 ID。null や内部 ID でない値は空。
func id(raw json.RawMessage) change.ID {
	var v change.ID
	if json.Unmarshal(raw, &v) != nil || !change.ValidID(v) {
		return ""
	}
	return v
}

func ids(list []json.RawMessage) []change.ID {
	var out []change.ID
	for _, raw := range list {
		if v := id(raw); v != "" {
			out = append(out, v)
		}
	}
	return out
}
