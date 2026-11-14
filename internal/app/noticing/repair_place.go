package noticing

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// placeKind は、場所を変えた修復の種類(C4b2-8)。
type placeKind int

const (
	placeNone      placeKind = iota
	placeWorkspace           // 親と別のワークスペース
	placeCycle               // 親の循環
	placeProject             // 親と別のプロジェクト
	placeNumber              // 番号の重なり
)

// placeMove は、修復の保存で set place の行を書いた、アイテム・プロジェクト 1 つ。before は修復の直前のログ。
// プロジェクトの場所は、ワークスペースと番号だけを持つ item.Place で表す。
type placeMove struct {
	ref      change.Ref
	from, to item.Place
	rows     []change.Record // 修復の保存の、このエンティティの行
	before   []change.Record
}

// places は、i 番目の修復の保存の、場所を変えた修復の通知(06「番号の重なりの修復」「親が循環したとき」
// 「親と別のワークスペース・プロジェクトにある子」)。set place の行を書いた、削除されていないアイテム・
// プロジェクトごとに 1 件(C4b2-8。親と別のワークスペースの保存で一緒に動いた削除済みの下のものには作らない)。
func places(r port.Reader, b batch, i int, me string) ([]port.Notice, error) {
	var out []port.Notice
	for _, e := range b.saves[i].Entities {
		m, ok := placeMoveOf(b, i, e)
		if !ok {
			continue
		}
		n, ok, err := m.notice(r, b, i, me)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, n)
		}
	}
	return out, nil
}

// placeMoveOf は、保存のエンティティ e の set place の行の、前と後の場所。行がないか、読めないか、修復の直前に
// 削除済みなら false。
func placeMoveOf(b batch, i int, e port.EntityRows) (placeMove, bool) {
	m := placeMove{ref: e.Ref, rows: e.Rows, before: b.beforeSave(e.Ref, i)}
	var row *change.Record
	for j := range e.Rows {
		if e.Rows[j].T == change.Set && e.Rows[j].F == "place" {
			row = &e.Rows[j]
		}
	}
	if row == nil {
		return placeMove{}, false
	}
	switch e.Ref.Kind {
	case change.KindItem:
		it, ok := item.FromLog(m.before)
		if !ok || it.Deleted != nil || json.Unmarshal(row.From, &m.from) != nil || json.Unmarshal(row.To, &m.to) != nil {
			return placeMove{}, false
		}
	case change.KindProject:
		p, ok := project.FromLog(m.before)
		var from, to project.Place
		if !ok || p.Deleted != nil || json.Unmarshal(row.From, &from) != nil || json.Unmarshal(row.To, &to) != nil {
			return placeMove{}, false
		}
		m.from = item.Place{Workspace: from.Workspace, Number: from.Number}
		m.to = item.Place{Workspace: to.Workspace, Number: to.Number}
	default:
		return placeMove{}, false
	}
	return m, true
}

// kind は、前と後の場所を、ワークスペース・親・プロジェクト・番号の順に比べた修復の種類(C4b2-8)。
// プロジェクトは番号だけを見る。
func (m placeMove) kind() placeKind {
	switch {
	case m.ref.Kind == change.KindProject && m.from.Workspace != m.to.Workspace:
		return placeNone
	case m.from.Workspace != m.to.Workspace:
		return placeWorkspace
	case !sameParent(m.from.Parent, m.to.Parent):
		return placeCycle
	case m.from.Project != m.to.Project:
		return placeProject
	case m.from.Number != m.to.Number:
		return placeNumber
	}
	return placeNone
}

func sameParent(a, b *item.ID) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

// recipient は、通知の宛先(C4b2-7)。番号は作成の行の by、ほかは場所を最後に保存した人。
func (m placeMove) recipient(k placeKind) string {
	if k == placeNumber {
		c, _ := change.First(m.before, change.Create, "")
		return c.By
	}
	return lastSaver(m.before, "place")
}

// notice は、m の通知。宛先があなたでないか、呼び方を決められなければ false。
func (m placeMove) notice(r port.Reader, b batch, i int, me string) (port.Notice, bool, error) {
	k := m.kind()
	if k == placeNone || !same(m.recipient(k), me) {
		return port.Notice{}, false, nil
	}
	self, ok, err := entityOf(r, b, m.ref)
	if !ok {
		return port.Notice{}, false, err
	}
	var text, key string
	switch k {
	case placeNumber:
		text, key, ok, err = m.numberText(r, b)
	case placeCycle:
		text, key, ok, err = m.cycleText(r, b, self.text)
	case placeProject:
		text, key, ok, err = m.projectText(r, b, self.text)
	default:
		text, key, ok, err = m.workspaceText(r, b, i)
	}
	if !ok {
		return port.Notice{}, false, err
	}
	n := self.base
	n.Text, n.Source = text, key
	return n, true, nil
}

// display は、接頭辞 prefix・番号 n の画面の ID。
func (m placeMove) display(prefix string, n int) string {
	if m.ref.Kind == change.KindProject {
		return project.Project{Place: project.Place{Number: n}}.DisplayID(prefix)
	}
	return item.Item{Place: item.Place{Number: n}}.DisplayID(prefix)
}

// numberText は、番号の重なりの本文と鍵。{旧 ID}・{新 ID} の接頭辞は、いまのワークスペースのもの。
func (m placeMove) numberText(r port.Reader, b batch) (string, string, bool, error) {
	w, ok, err := workspaceOf(r, b, m.to.Workspace)
	if !ok {
		return "", "", false, err
	}
	text := "2 台で同時に " + withParticle(m.display(w.Prefix, m.from.Number), "が") + "作られたため、あとから届いた方を " +
		withParticle(m.display(w.Prefix, m.to.Number), "に") + "振り直しました"
	return text, "repair:number:" + string(m.ref.ID) + ":" + strconv.Itoa(m.to.Number), true, nil
}

// cycleText は、親の循環の本文と鍵。行き先は新しい親、なければプロジェクト。Step を Task にしたら末尾を足す。
func (m placeMove) cycleText(r port.Reader, b batch, self string) (string, string, bool, error) {
	dest := change.Ref{Kind: change.KindProject, ID: m.to.Project}
	if m.to.Parent != nil {
		dest = change.Ref{Kind: change.KindItem, ID: *m.to.Parent}
	}
	d, ok, err := entityOf(r, b, dest)
	if !ok {
		return "", "", false, err
	}
	text := "親が循環したため、" + withParticle(self, "を") + " " + withParticle(d.text, "の") + "下へ移しました"
	if m.toTask() {
		text += "。Step は Task になりました"
	}
	return text, "repair:cycle:" + string(m.ref.ID) + ":" + string(dest.ID), true, nil
}

// toTask は、同じ保存で Step を Task にしたか。
func (m placeMove) toTask() bool {
	for _, row := range m.rows {
		var from, to item.Kind
		if row.T == change.Set && row.F == "kind" && json.Unmarshal(row.From, &from) == nil &&
			json.Unmarshal(row.To, &to) == nil && from == item.Step && to == item.Task {
			return true
		}
	}
	return false
}

// placeKey は、親と別のワークスペース・プロジェクトの鍵(C4b2-6)。
func (m placeMove) placeKey() string {
	return "repair:place:" + string(m.ref.ID) + ":" + string(m.to.Workspace) + ":" + string(m.to.Project)
}

// projectText は、親と別のプロジェクトの本文と鍵(C4b2-9 の空白)。
func (m placeMove) projectText(r port.Reader, b batch, self string) (string, string, bool, error) {
	if m.to.Parent == nil {
		return "", "", false, nil
	}
	parent, ok, err := entityOf(r, b, change.Ref{Kind: change.KindItem, ID: *m.to.Parent})
	if !ok {
		return "", "", false, err
	}
	log, err := logOf(r, b, change.Ref{Kind: change.KindProject, ID: m.to.Project})
	if err != nil {
		return "", "", false, err
	}
	p, ok := project.FromLog(log)
	if !ok {
		return "", "", false, nil
	}
	text := withParticle(parent.text, "が") + " プロジェクト「" + oneLine(p.Title) + "」へ移動したため、" +
		withParticle(self, "を") + "そのプロジェクトに移しました"
	return text, m.placeKey(), true, nil
}

// workspaceText は、親と別のワークスペースの本文と鍵。{旧 ID} は移動元、{新 ID} は移動先の、いまの接頭辞。
// 空になった項目があれば末尾に足す。
func (m placeMove) workspaceText(r port.Reader, b batch, i int) (string, string, bool, error) {
	if m.to.Parent == nil {
		return "", "", false, nil
	}
	parent, ok, err := movedParentID(r, b, *m.to.Parent)
	if !ok {
		return "", "", false, err
	}
	src, ok, err := workspaceOf(r, b, m.from.Workspace)
	if !ok {
		return "", "", false, err
	}
	dst, ok, err := workspaceOf(r, b, m.to.Workspace)
	if !ok {
		return "", "", false, err
	}
	emptied, err := m.emptied(r, b, i)
	if err != nil {
		return "", "", false, err
	}
	text := withParticle(parent, "が") + " " + withParticle(oneLine(dst.Name), "へ") + "移動したため、" +
		withParticle(m.display(src.Prefix, m.from.Number), "を") + " " + withParticle(m.display(dst.Prefix, m.to.Number), "に") + "移動しました"
	if len(emptied) > 0 {
		text += "。合う値がなく空になった項目: " + strings.Join(emptied, "・")
	}
	return text, m.placeKey(), true, nil
}

// movedParentID は、親と別のワークスペースの本文の {親の ID}。親が移動先のワークスペースへ入る前の ID で、
// {旧 ID} と対になる。同じ Store の中の、親の最後の転送記録の old を使い、なければ親のいまの表示。
func movedParentID(r port.Reader, b batch, parent item.ID) (string, bool, error) {
	recs, err := r.Transfers().List()
	if err = notFoundIsSkip(err); err != nil {
		return "", false, err
	}
	for j := len(recs) - 1; j >= 0; j-- {
		c := recs[j]
		if c.Entity == transfer.EntityItem && c.Target == parent && c.ToStore == string(b.store) && c.Old != "" {
			return c.Old, true, nil
		}
	}
	e, ok, err := entityOf(r, b, change.Ref{Kind: change.KindItem, ID: parent})
	return e.text, ok, err
}

// emptied は、修復の直前のアイテムに、移動元・移動先の修復の直前の設定と、移動先のプロジェクトのルールで
// transfer.Remap をかけ直した、空になった項目(C3b-7)。設定を作れなければなし。
func (m placeMove) emptied(r port.Reader, b batch, i int) ([]string, error) {
	it, ok := item.FromLog(m.before)
	if !ok {
		return nil, nil
	}
	fromLog, err := beforeRead(r, b, change.Ref{Kind: change.KindWorkspace, ID: m.from.Workspace}, i)
	if err != nil {
		return nil, err
	}
	toLog, err := beforeRead(r, b, change.Ref{Kind: change.KindWorkspace, ID: m.to.Workspace}, i)
	if err != nil {
		return nil, err
	}
	from, ok := workspace.SettingsFromLog(fromLog)
	if !ok {
		return nil, nil
	}
	to, ok := workspace.SettingsFromLog(toLog)
	if !ok {
		return nil, nil
	}
	return transfer.Remap(it, from, to, projectrule.FromLog(toLog).Of(m.to.Project), nil, nil).Emptied, nil
}
