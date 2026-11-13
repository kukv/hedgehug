package transfer

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// custom は、追加プロパティ p の値を、移動先の同じキー・同じ型のプロパティの欄に書き直し、移動元の欄を空にする。
// 欄の名前(fields/<プロパティの内部 ID>)がワークスペースごとに違うので、残る値でも必ず書き直す。
func (r *remapper) custom(p workspace.Property) {
	to, found := r.counterpart(p)
	switch p.Type {
	case workspace.TypeSingle:
		r.single(p, to, found)
	case workspace.TypeMulti:
		r.multi(p, to, found)
	default:
		r.plain(p, to, found)
	}
}

// counterpart は、移動先の、組み込みでなく、キーと型が p と同じプロパティ。
func (r *remapper) counterpart(p workspace.Property) (workspace.Property, bool) {
	i := slices.IndexFunc(r.to.Properties, func(t workspace.Property) bool {
		return !t.Builtin && t.Key == p.Key && t.Type == p.Type
	})
	if i < 0 {
		return workspace.Property{}, false
	}
	return r.to.Properties[i], true
}

// plain は、テキスト・数値・日付・チェック・URL・ユーザー。
func (r *remapper) plain(p, to workspace.Property, found bool) {
	raw := r.it.Fields[p.ID]
	if !hasValue(p, raw) {
		return
	}
	if !found {
		r.draft(change.NewSet(item.FieldKey(p.ID), raw, nil))
		r.empty(p.Name)
		return
	}
	v := any(raw)
	if p.Type == workspace.TypeUser {
		v = r.person(raw)
	}
	r.draft(change.NewSet(item.FieldKey(to.ID), nil, v))
	r.draft(change.NewSet(item.FieldKey(p.ID), raw, nil))
}

// hasValue は、値が入っているか。チェックは true だけ、文字列は空でないものだけを入っているとする。
func hasValue(p workspace.Property, raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" || s == `""` {
		return false
	}
	return p.Type != workspace.TypeCheck || s != "false"
}

func (r *remapper) single(p, to workspace.Property, found bool) {
	vs := r.it.Choices[p.ID]
	if len(vs) == 0 {
		return
	}
	if v, ok := choiceMatch(p, to, found, vs[0]); ok {
		r.draft(change.NewSet(item.FieldKey(to.ID), nil, v))
	} else {
		r.empty(p.Name)
	}
	r.draft(change.NewSet(item.FieldKey(p.ID), vs[0], nil))
}

func (r *remapper) multi(p, to workspace.Property, found bool) {
	vs := r.it.Choices[p.ID]
	for _, v := range vs {
		r.draft(change.NewRemove(item.FieldKey(p.ID), v))
	}
	var added []change.ID
	for _, v := range vs {
		if m, ok := choiceMatch(p, to, found, v); ok && !slices.Contains(added, m) {
			added = append(added, m)
			r.draft(change.NewAdd(item.FieldKey(to.ID), m))
		}
	}
	if len(vs) > 0 && len(added) == 0 {
		r.empty(p.Name)
	}
}

// choiceMatch は、p の選択肢 id と名前が完全に一致する、移動先のプロパティ to の選択肢。
func choiceMatch(p, to workspace.Property, found bool, id change.ID) (change.ID, bool) {
	if !found {
		return "", false
	}
	return matchName(p.Choices, to.Choices, id)
}

func (r *remapper) assignee() {
	if r.it.Assignee == "" {
		return
	}
	if to := r.login(r.it.Assignee); to != r.it.Assignee {
		r.draft(change.NewSet("assignee", r.it.Assignee, to))
	}
}

// person は、ユーザー型の値(JSON の文字列)を移動先の綴りに置き換える。文字列でなければそのまま。
func (r *remapper) person(raw json.RawMessage) any {
	var login string
	if json.Unmarshal(raw, &login) != nil {
		return raw
	}
	return r.login(login)
}

// login は、別の Store なら移動先の綴り。同じ Store か、people にいなければそのまま。
func (r *remapper) login(login string) string {
	if r.people == nil {
		return login
	}
	if to, ok := r.people[strings.ToLower(login)]; ok {
		return to
	}
	return login
}

// watchers は、別の Store のとき、綴りの変わるウォッチを、取り除く行を先、足す行を後に書く。
// 置き換えたログインがすでにウォッチにいるときは、足さない。
func (r *remapper) watchers() {
	if r.people == nil {
		return
	}
	var kept, added []string
	for _, w := range r.it.Watchers {
		if r.login(w) == w {
			kept = append(kept, w)
		}
	}
	for _, w := range r.it.Watchers {
		if to := r.login(w); to != w {
			r.draft(change.NewRemove("watchers", w))
			if !slices.Contains(kept, to) && !slices.Contains(added, to) {
				added = append(added, to)
			}
		}
	}
	for _, to := range added {
		r.draft(change.NewAdd("watchers", to))
	}
}
