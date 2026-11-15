package repair

import (
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// IssueLink は、重なりを数える範囲(決定 C5c2-2)の Issue のリンク 1 本。
// Linking は、リンクした行(決定 C5c2-17。呼ぶ側が itemlink.Origin でたどって渡す)。
type IssueLink struct {
	Item    item.ID
	Ref     itemlink.Ref
	Linking change.Record
}

// IssueUnlinks は、外す Issue のリンクを並びの順に返す(決定 C5c2-4)。種別が Issue でないリンクは数えない。
// older で並べ、先頭から、そのアイテムも、その Issue(Ref.Same)も、まだ残すと決まっていなければ残し、
// どちらかが決まっていれば外す。重ならなければ空。
func IssueUnlinks(all []IssueLink, f Facts) []IssueLink {
	sorted := slices.DeleteFunc(slices.Clone(all), func(l IssueLink) bool { return l.Ref.Kind != itemlink.Issue })
	slices.SortStableFunc(sorted, older(
		func(l IssueLink) change.Record { return l.Linking },
		func(l IssueLink) string { return string(l.Linking.ID) },
		f,
	))
	keptItems := map[item.ID]bool{}
	var keptRefs []itemlink.Ref
	var out []IssueLink
	for _, l := range sorted {
		if keptItems[l.Item] || slices.ContainsFunc(keptRefs, l.Ref.Same) {
			out = append(out, l)
			continue
		}
		keptItems[l.Item] = true
		keptRefs = append(keptRefs, l.Ref)
	}
	return out
}

// Spelling は、メンバーの綴り 1 つ(Login)と、綴りを決めた行(Row。決定 C5c2-3)。
type Spelling struct {
	Login string
	Row   change.Record
}

// MemberMerge は、大文字小文字だけが違う綴りの 1 組。Keep を残し、Drop を外す。
type MemberMerge struct {
	Keep string
	Drop []string
}

// MemberMerges は、小文字にそろえて重なった綴りの組ごとの統合を返す(決定 C5c2-6)。
// 残す綴りは keeper で決め(tie は行の ID)、組は小文字のログイン名の順、Drop は綴りの辞書順。重ならなければ空。
func MemberMerges(all []Spelling, f Facts) []MemberMerge {
	groups := map[string][]Spelling{}
	for _, s := range all {
		k := strings.ToLower(s.Login)
		groups[k] = append(groups[k], s)
	}
	keys := make([]string, 0, len(groups))
	for k, g := range groups {
		if len(g) > 1 {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	var out []MemberMerge
	for _, k := range keys {
		g := groups[k]
		keep := keeper(g, func(s Spelling) change.Record { return s.Row }, func(s Spelling) string { return string(s.Row.ID) }, f)
		m := MemberMerge{Keep: keep.Login}
		for _, s := range g {
			if s.Login != keep.Login {
				m.Drop = append(m.Drop, s.Login)
			}
		}
		slices.Sort(m.Drop)
		out = append(out, m)
	}
	return out
}
