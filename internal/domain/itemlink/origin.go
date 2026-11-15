package itemlink

import (
	"encoding/json"
	"slices"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// Origin は、add が改名・移管を追った行(Set.Rename の add。同じ op に、種別と番号が同じで別のリポジトリの
// remove links がある)なら、その remove の prev が指す旧い Ref の add の行へ、改名でない行に着くまでたどって返す
// (C5c2-17)。旧い add が複数なら change.ListRow の勝った行。add が改名の行でなければ add のまま。
// たどった先が見つからなければ、そこまでの行。prev が輪になっていても、たどった行を覚えて止まる。
func Origin(log []change.Record, add change.Record) change.Record {
	seen := map[change.ID]bool{}
	cur := add
	for !seen[cur.ID] {
		seen[cur.ID] = true
		next, ok := renamedFrom(log, cur)
		if !ok {
			break
		}
		cur = next
	}
	return cur
}

// renamedFrom は、add が改名の行なら、旧い Ref の add の行(複数なら勝った行)を返す。
func renamedFrom(log []change.Record, add change.Record) (change.Record, bool) {
	to, ok := refOf(add.To)
	if !ok || add.Op == "" {
		return change.Record{}, false
	}
	var olds []change.Record
	var from json.RawMessage
	for _, r := range log {
		if r.Op != add.Op || r.T != change.Remove || r.F != LinksField {
			continue
		}
		if old, ok := refOf(r.From); ok && old.Kind == to.Kind && old.Number == to.Number && old.Repo != to.Repo {
			olds = append(olds, addsIn(log, r.Prev)...)
			from = r.From
		}
	}
	if len(olds) == 0 {
		return change.Record{}, false
	}
	return change.ListRow(olds, LinksField, from)
}

// addsIn は、log の add links の行のうち、ids に挙がっているもの。
func addsIn(log []change.Record, ids []change.ID) []change.Record {
	var out []change.Record
	for _, x := range log {
		if x.T == change.Add && x.F == LinksField && slices.Contains(ids, x.ID) {
			out = append(out, x)
		}
	}
	return out
}
