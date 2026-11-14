package transfer

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// PeopleMap は、移動元のログイン名(小文字)から移動先のログイン名への対応。
type PeopleMap map[string]string

// People は、移すアイテムに出てくる人を数え、移動先のメンバーに足す人(小文字のログイン名の順)と、数えた全員の対応を返す。
// 数えるのは、担当者・ユーザー型の値・ウォッチ・作成者・logs の行の by・extra(プロジェクトのオーナーや専用ルールの担当者など)。空・hedgehug[bot]・ログイン名に使えない文字の人は数えない。
// 大文字小文字だけが違う人は同じ人。移動先にいる人は移動先の綴りに置き換え、いない人は足す。
// 足す人の綴りは、移動元のメンバーにいればその綴り、いなければ見つけた綴りのうち辞書順で先頭。表示名は移動元のメンバーの表示名(いなければログイン名)。
func People(from workspace.Settings, items []item.Item, logs [][]change.Record, extra []string, fromMembers, toMembers []member.Member) ([]member.Member, PeopleMap) {
	found := mentioned(from, items, logs, extra)

	people := PeopleMap{}
	var add []member.Member
	for _, key := range slices.Sorted(maps.Keys(found)) {
		if to, ok := memberOf(toMembers, key); ok {
			people[key] = to.Login
			continue
		}
		m := newMember(key, found[key], fromMembers)
		people[key] = m.Login
		add = append(add, m)
	}
	return add, people
}

// mentioned は、数える人を、小文字のログイン名ごとに、見つけた綴りにまとめる。
func mentioned(from workspace.Settings, items []item.Item, logs [][]change.Record, extra []string) map[string][]string {
	found := map[string][]string{} // 小文字 → 見つけた綴り
	count := func(login string) {
		if login != "" && login != change.BotLogin && member.ValidLogin(login) {
			key := strings.ToLower(login)
			found[key] = append(found[key], login)
		}
	}
	userProps := userProperties(from)
	for _, it := range items {
		count(it.Assignee)
		for _, w := range it.Watchers {
			count(w)
		}
		count(it.Created.By)
		for _, id := range userProps {
			var login string
			if json.Unmarshal(it.Fields[id], &login) == nil {
				count(login)
			}
		}
	}
	for _, login := range extra {
		count(login)
	}
	for _, log := range logs {
		for _, r := range log {
			count(r.By)
		}
	}
	return found
}

// userProperties は、ユーザー型の追加プロパティの内部 ID。
func userProperties(s workspace.Settings) []change.ID {
	var ids []change.ID
	for _, p := range s.Properties {
		if p.Type == workspace.TypeUser && !p.Builtin {
			ids = append(ids, p.ID)
		}
	}
	return ids
}

func memberOf(ms []member.Member, lower string) (member.Member, bool) {
	i := slices.IndexFunc(ms, func(m member.Member) bool { return strings.ToLower(m.Login) == lower })
	if i < 0 {
		return member.Member{}, false
	}
	return ms[i], true
}

// newMember は、移動先に足す人。
func newMember(lower string, spellings []string, fromMembers []member.Member) member.Member {
	if m, ok := memberOf(fromMembers, lower); ok {
		return m
	}
	login := slices.Min(spellings)
	return member.Member{Login: login, Name: login}
}
