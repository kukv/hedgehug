package noticing

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// restores は、i 番目の修復の保存の、削除の取り消しの通知(06「削除と変更の重なり」)。restore の行の to の
// 削除の op ごとに 1 件で、宛先は削除の行を書いた人(C4b2-7・C4b2-8)。
func restores(r port.Reader, b batch, i int, me string) ([]port.Notice, error) {
	var out []port.Notice
	for _, op := range restoredOps(b.saves[i]) {
		ref, d, ok := deletionOf(b, i, op)
		if !ok || !same(d.By, me) {
			continue
		}
		before := b.beforeSave(ref, i)
		target, ok, err := restoreTarget(r, b, ref, before, d)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		n := target.base
		n.Text = withParticle(target.text, "は") + " " + memberName(r, repair.Changer(before, d)) + " が同時に変更したため、削除を取り消しました"
		n.Source = "repair:restore:" + string(op)
		out = append(out, n)
	}
	return out, nil
}

// restoredOps は、保存の restore の行の to(取り消した削除の op)を、出た順に重なりなく返す。
func restoredOps(s port.ReceivedSave) []change.ID {
	var out []change.ID
	for _, e := range s.Entities {
		for _, row := range e.Rows {
			var op change.ID
			if row.T != change.Restore || json.Unmarshal(row.To, &op) != nil || slices.Contains(out, op) {
				continue
			}
			out = append(out, op)
		}
	}
	return out
}

// deletionOf は、i 番目の保存のエンティティのうち、保存の直前のログに、op の取り消しの対象の削除の行
// (root: true のエンティティの削除か、deleted でない f の削除)があるものと、その行。1 つの op で複数を削除して
// いたら(テンプレートの子など)、重なった変更(repair.LastChange)が最もあとのものを取る(重なった変更があった方が
// 取り消しの理由。repair.Overlaps は片側の見分けで選ぶが、ここにはその材料がない)。同じなら f の辞書順で最初のもの。
func deletionOf(b batch, i int, op change.ID) (change.Ref, change.Record, bool) {
	var best *restoreCandidate
	for _, e := range b.saves[i].Entities {
		before := b.beforeSave(e.Ref, i)
		for _, c := range before {
			if c.T != change.Delete || c.Op != op || (c.F == change.DeletedField && (c.Root == nil || !*c.Root)) {
				continue
			}
			w, changed := repair.LastChange(before, c)
			cand := restoreCandidate{e.Ref, c, w, changed}
			if best == nil || cand.better(*best) {
				best = &cand
			}
		}
	}
	if best == nil {
		return change.Ref{}, change.Record{}, false
	}
	return best.ref, best.d, true
}

// restoreCandidate は、取り消しの対象の候補の削除の行と、それと重なった最もあとの変更。
type restoreCandidate struct {
	ref     change.Ref
	d       change.Record
	w       change.Record
	changed bool
}

// better は、c が o より取り消しの対象らしいか。重なった変更があり、それがあとのもの。同じなら f の辞書順。
func (c restoreCandidate) better(o restoreCandidate) bool {
	if c.changed != o.changed {
		return c.changed
	}
	if c.changed && !c.w.At.Equal(o.w.At) {
		return c.w.At.After(o.w.At)
	}
	return c.d.F < o.d.F
}

// restoreTarget は、取り消した削除 d の {対象} と下地(C4b2-9・C4b2-10)。エンティティの削除はその ID か
// ワークスペースの名前、ワークスペースの設定の値・スプリント・リリース・ルールは「「{名前}」」、アイテム・
// プロジェクトのログの f の削除はコメントか作業時間。
func restoreTarget(r port.Reader, b batch, ref change.Ref, before []change.Record, d change.Record) (entity, bool, error) {
	if d.F == change.DeletedField {
		return entityOf(r, b, ref)
	}
	if ref.Kind == change.KindWorkspace {
		name := "「" + oneLine(workspace.NameOfDeleted(before, d.F)) + "」"
		return entity{name, port.Notice{Workspace: ref.ID, Section: section(d.F)}}, true, nil
	}
	owner, ok, err := entityOf(r, b, ref)
	if !ok {
		return entity{}, false, err
	}
	owner.text += " の" + recordName(before, d.F)
	return owner, true, nil
}

// recordName は、アイテム・プロジェクトのログの f の記録の呼び方。作業時間の行があれば作業時間、ほかはコメント。
func recordName(log []change.Record, f string) string {
	if _, ok := change.First(log, change.Worklog, f); ok {
		return "作業時間"
	}
	return "コメント"
}

// sections は、ワークスペースのログの f の頭と、開く区画(C4b2-10)。
var sections = []struct{ prefix, section string }{
	{"statuses/", "statuses"},
	{"priorities/", "priorities"},
	{"systems/", "systems"},
	{"properties/", "properties"},
	{"templates/", "templates"},
	{"rules/", "automation"},
	{"sprints/", "sprints"},
	{"releases/", "releases"},
}

// section は、ワークスペースのログの f を開く区画。どれでもなければ general。
func section(f string) string {
	for _, s := range sections {
		if strings.HasPrefix(f, s.prefix) {
			return s.section
		}
	}
	return "general"
}

// cascades は、i 番目の修復の保存の、親の削除の通知(06「親の削除」)。root: false の削除の行を書いた子ごとに
// 1 件で、宛先は修復の直前に子へ最後に保存した人(C4b2-7)。
func cascades(r port.Reader, b batch, i int, me string) ([]port.Notice, error) {
	var out []port.Notice
	for _, e := range b.saves[i].Entities {
		for _, row := range e.Rows {
			if row.T != change.Delete || row.F != change.DeletedField || row.Root == nil || *row.Root {
				continue
			}
			n, ok, err := cascade(r, b, i, e.Ref, row.Op, me)
			if err != nil {
				return nil, err
			}
			if ok {
				out = append(out, n)
			}
		}
	}
	return out, nil
}

// cascade は、子 child を削除の op で削除済みに入れた修復の通知。親は、同じ op で root: true の削除の行を持つ祖先。
func cascade(r port.Reader, b batch, i int, child change.Ref, op change.ID, me string) (port.Notice, bool, error) {
	if !same(lastSaver(b.beforeSave(child, i), ""), me) {
		return port.Notice{}, false, nil
	}
	c, ok, err := entityOf(r, b, child)
	if !ok {
		return port.Notice{}, false, err
	}
	p, ok, err := rootOf(r, b, child, op)
	if !ok {
		return port.Notice{}, false, err
	}
	n := c.base
	n.Text = withParticle(p.text, "が") + "削除されたため、" + withParticle(c.text, "も") + "削除済みに入りました"
	n.Source = "repair:cascade:" + string(child.ID) + ":" + string(op)
	return n, true, nil
}

// rootOf は、child の祖先を近い方から辿り、op で root: true の削除の行を持つものの呼び方。なければ false。
func rootOf(r port.Reader, b batch, child change.Ref, op change.ID) (entity, bool, error) {
	seen := map[change.Ref]bool{child: true}
	ref := child
	for {
		log, err := logOf(r, b, ref)
		if err != nil {
			return entity{}, false, err
		}
		if ref != child && slices.ContainsFunc(log, func(c change.Record) bool {
			return c.T == change.Delete && c.F == change.DeletedField && c.Op == op && c.Root != nil && *c.Root
		}) {
			return entityOf(r, b, ref)
		}
		next, ok := parentOf(ref, log)
		if !ok || seen[next] {
			return entity{}, false, nil
		}
		seen[next], ref = true, next
	}
}

// parentOf は、アイテム・プロジェクト ref の、すぐ上のもの(親のアイテム、プロジェクト、ワークスペース)。
func parentOf(ref change.Ref, log []change.Record) (change.Ref, bool) {
	switch ref.Kind {
	case change.KindItem:
		it, ok := item.FromLog(log)
		switch {
		case !ok:
			return change.Ref{}, false
		case it.Place.Parent != nil:
			return change.Ref{Kind: change.KindItem, ID: *it.Place.Parent}, true
		case it.Place.Project != "":
			return change.Ref{Kind: change.KindProject, ID: it.Place.Project}, true
		}
		return change.Ref{Kind: change.KindWorkspace, ID: it.Place.Workspace}, true
	case change.KindProject:
		if p, ok := project.FromLog(log); ok {
			return change.Ref{Kind: change.KindWorkspace, ID: p.Place.Workspace}, true
		}
	}
	return change.Ref{}, false
}
