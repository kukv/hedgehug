package transfer

import (
	"slices"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// RemapResult は、Remap が書く行と、値が入っていたのに空になった項目の名前。
type RemapResult struct {
	Drafts  []change.Draft
	Emptied []string // 移動元のプロパティの名前。from.Properties の並びで、同じ名前は 1 回
}

// UsableStatuses は、移動先のプロジェクトが使うステータスの内部 ID(並びのまま)。toRules は移動先のプロジェクトの
// プロジェクト別のルールで、使うかの判定はプロジェクト別のルール(B5)の projectrule.UsedStatuses に任せる
// (外したステータスは使わない。初期ステータスと分類ごとの既定は外す印があっても使う)。
func UsableStatuses(to workspace.Settings, toRules projectrule.Rules) []change.ID {
	used := projectrule.UsedStatuses(to, toRules)
	ids := make([]change.ID, 0, len(used))
	for _, v := range used {
		ids = append(ids, v.ID)
	}
	return ids
}

// Unmatched は、statuses のうち、移動元にいて、名前が移動先で使えるどのステータスの名前とも一致しないものを、
// statuses の並びのまま返す。移動元にいない内部 ID は返さない(Remap は移動先の初期ステータスにする)。
func Unmatched(statuses []change.ID, from, to workspace.Settings, toRules projectrule.Rules) []change.ID {
	var out []change.ID
	for _, id := range statuses {
		v, ok := valueByID(from.Statuses, id)
		if !ok {
			continue
		}
		if _, ok := usableByName(to, toRules, v.Name); !ok {
			out = append(out, id)
		}
	}
	return out
}

// Remap は、アイテム 1 件の値を、移動先のワークスペースの定義に引き直す(02「値の引き直し」)。
// 行の並びは from.Properties の並びでプロパティごと(組み込みが先、続いて追加プロパティ)、スプリントとリリースを外す行、
// ウォッチは最後。
// people が nil なら同じ Store(担当者・ユーザー型・ウォッチはそのまま)、nil でなければ別の Store。
// toRules は移動先のプロジェクトのプロジェクト別のルール(UsableStatuses)。statusChoice は、移動先で使えない
// 移動元のステータス → 選んだ置き換え先。
func Remap(it item.Item, from, to workspace.Settings, toRules projectrule.Rules, people PeopleMap, statusChoice map[change.ID]change.ID) RemapResult {
	r := &remapper{it: it, from: from, to: to, rules: toRules, people: people, choice: statusChoice}
	for _, p := range from.Properties {
		if p.Builtin {
			r.builtin(p)
		} else {
			r.custom(p)
		}
	}
	r.plan()
	r.watchers()
	return r.res
}

type remapper struct {
	it       item.Item
	from, to workspace.Settings
	rules    projectrule.Rules
	people   PeopleMap
	choice   map[change.ID]change.ID
	res      RemapResult
}

func (r *remapper) draft(d change.Draft) { r.res.Drafts = append(r.res.Drafts, d) }

// empty は、空になった項目として name を並べる(同じ名前は 1 回)。
func (r *remapper) empty(name string) {
	if !slices.Contains(r.res.Emptied, name) {
		r.res.Emptied = append(r.res.Emptied, name)
	}
}

// plan は、スプリントとリリースを外す(02「移動先で変わること」。ワークスペースごとのものなので移動先では使えない)。
func (r *remapper) plan() {
	if r.it.Sprint != "" {
		r.draft(change.NewSet(item.SprintField, r.it.Sprint, nil))
	}
	if r.it.Release != "" {
		r.draft(change.NewSet(item.ReleaseField, r.it.Release, nil))
	}
}

func (r *remapper) builtin(p workspace.Property) {
	switch p.Key {
	case "status":
		r.status()
	case "assignee":
		r.assignee()
	case "priority":
		r.priority(p)
	case "systems":
		r.systems(p)
	}
}

func (r *remapper) status() {
	if r.it.Status == "" {
		return
	}
	r.draft(change.NewSet("status", r.it.Status, r.statusTarget(r.it.Status)))
}

// statusTarget は、移動先のステータス。名前が移動先で使えるものと一致すればそれ、なければ選んだ置き換え先
// (使えるものにあるときだけ)、なければ元の分類の既定。移動元にいなければ初期ステータス。
func (r *remapper) statusTarget(id change.ID) change.ID {
	v, ok := valueByID(r.from.Statuses, id)
	if !ok {
		return r.to.InitialStatus
	}
	if to, ok := usableByName(r.to, r.rules, v.Name); ok {
		return to
	}
	if c, ok := r.choice[id]; ok && slices.Contains(UsableStatuses(r.to, r.rules), c) {
		return c
	}
	if d := r.to.CategoryDefaults[v.Category]; d != "" {
		return d
	}
	return r.to.InitialStatus
}

func (r *remapper) priority(p workspace.Property) {
	if r.it.Priority == "" {
		return
	}
	to, ok := matchName(r.from.Priorities, r.to.Priorities, r.it.Priority)
	if !ok {
		to = r.to.InitialPriority
		r.empty(p.Name)
	}
	r.draft(change.NewSet("priority", r.it.Priority, to))
}

func (r *remapper) systems(p workspace.Property) {
	for _, id := range r.it.Systems {
		r.draft(change.NewRemove("systems", id))
	}
	var added []change.ID
	for _, id := range r.it.Systems {
		if to, ok := matchName(r.from.Systems, r.to.Systems, id); ok && !slices.Contains(added, to) {
			added = append(added, to)
			r.draft(change.NewAdd("systems", to))
		}
	}
	if len(r.it.Systems) > 0 && len(added) == 0 {
		r.empty(p.Name)
	}
}

// valueByID は、vs から内部 ID が id の値を引く。
func valueByID(vs []workspace.Value, id change.ID) (workspace.Value, bool) {
	i := slices.IndexFunc(vs, func(v workspace.Value) bool { return v.ID == id })
	if i < 0 {
		return workspace.Value{}, false
	}
	return vs[i], true
}

// matchName は、from の値 id と名前が完全に一致する to の値の内部 ID(大文字小文字も区別する)。
func matchName(from, to []workspace.Value, id change.ID) (change.ID, bool) {
	v, ok := valueByID(from, id)
	if !ok {
		return "", false
	}
	i := slices.IndexFunc(to, func(t workspace.Value) bool { return t.Name == v.Name })
	if i < 0 {
		return "", false
	}
	return to[i].ID, true
}

// usableByName は、名前が name と完全に一致する、移動先で使えるステータスの内部 ID。
func usableByName(to workspace.Settings, rules projectrule.Rules, name string) (change.ID, bool) {
	usable := UsableStatuses(to, rules)
	for _, v := range to.Statuses {
		if v.Name == name && slices.Contains(usable, v.ID) {
			return v.ID, true
		}
	}
	return "", false
}
