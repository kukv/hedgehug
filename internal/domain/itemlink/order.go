package itemlink

import (
	"cmp"
	"encoding/json"
	"slices"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// valid は、Store から読んだリンクの値が検査に合うか(種別が PR か Issue、リポジトリが owner/name の形、
// 番号が 1 以上)。合わない行は無いものとして扱う。
func (r Ref) valid() bool {
	if r.Kind != PR && r.Kind != Issue || r.Number <= 0 {
		return false
	}
	repo, err := project.NormalizeRepo(r.Repo)
	return err == nil && repo == r.Repo
}

// refOf は、行の値をリンクとして読む。検査に合わなければ false。
func refOf(raw json.RawMessage) (Ref, bool) {
	var r Ref
	if json.Unmarshal(raw, &r) != nil || !r.valid() {
		return Ref{}, false
	}
	return r, true
}

// place は、リンクの並びの鍵(add の行の時刻と ID)。
type place struct {
	at time.Time
	id change.ID
}

func (p place) compare(o place) int {
	return cmp.Or(p.at.Compare(o.at), cmp.Compare(p.id, o.id))
}

// placesOf は、各リンクの並びの鍵: 最後の add の行の (At, ID)(ログの行の位置によらない)。改名・移管
// (同じ op に、同じ種別・番号の links の remove がある add)は、外したリンクの鍵を引き継ぐ(D3-2・D3-11)。
func placesOf(log []change.Record) map[Ref]place {
	rows := slices.Clone(log)
	slices.SortStableFunc(rows, func(a, b change.Record) int { return place{a.At, a.ID}.compare(place{b.At, b.ID}) })
	removed := map[change.ID][]Ref{}
	for _, r := range rows {
		if ref, ok := refOf(r.From); ok && r.T == change.Remove && r.F == LinksField {
			removed[r.Op] = append(removed[r.Op], ref)
		}
	}
	out := map[Ref]place{}
	for _, r := range rows {
		ref, ok := refOf(r.To)
		if !ok || r.T != change.Add || r.F != LinksField {
			continue
		}
		out[ref] = place{r.At, r.ID}
		for _, old := range removed[r.Op] {
			if p, had := out[old]; had && old != ref && old.Kind == ref.Kind && old.Number == ref.Number {
				out[ref] = p
			}
		}
	}
	return out
}
