package noticing

import (
	"encoding/json"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// purgedUnder は、i 番目の修復の保存が修復 (1) の完全な削除(行がなく、消えたエンティティがある)なら、消えた
// アイテムごとの通知(06「完全な削除と重なったとき」)。宛先は消えたアイテムのログで最後に保存した人(C4b2-7・C4b2-8)。
func purgedUnder(r port.Reader, b batch, i int, me string) ([]port.Notice, error) {
	s := b.saves[i]
	if !s.Repair || len(s.Entities) > 0 {
		return nil, nil
	}
	var out []port.Notice
	for _, e := range s.Removed {
		if e.Ref.Kind != change.KindItem || !same(lastSaver(e.Log, ""), me) {
			continue
		}
		n, ok, err := purgedUnderNotice(r, b, s.Removed, e)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, n)
		}
	}
	return out, nil
}

// purgedUnderNotice は、修復 (1) で消えたアイテム e の通知。{ID} は、完全に削除された祖先(近い方から探す)の
// 記録の ID。見つからないか、e を読めなければ false。
func purgedUnderNotice(r port.Reader, b batch, removed []port.RemovedEntity, e port.RemovedEntity) (port.Notice, bool, error) {
	parent, ok := parentOf(e.Ref, e.Log)
	if !ok {
		return port.Notice{}, false, nil
	}
	c, ok, err := findCause(r, b, removed, parent)
	if !ok || c.purged == nil {
		return port.Notice{}, false, err
	}
	g, ok, err := goneOf(r, b, e.Ref, e.Log)
	if !ok {
		return port.Notice{}, false, err
	}
	n := g.base
	n.Text, n.Source = purgedText(c.text), "repair:purged:"+string(e.Ref.ID)
	return n, true, nil
}

// purgedRelations は、i 番目の修復の保存の、完全に削除された相手への関連を外した通知。外した関連ごとに 1 件で、
// 宛先は修復の直前のログでその関連を最後に足した人(C4b2-7)。
func purgedRelations(r port.Reader, b batch, i int, me string) ([]port.Notice, error) {
	var out []port.Notice
	for _, e := range b.saves[i].Entities {
		for _, row := range e.Rows {
			var rel item.Relation
			if e.Ref.Kind != change.KindItem || row.T != change.Remove || row.F != "relations" ||
				json.Unmarshal(row.From, &rel) != nil || !same(relationAdder(b.beforeSave(e.Ref, i), rel), me) {
				continue
			}
			n, ok, err := relationNotice(r, b, e.Ref, rel.To)
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

// relationNotice は、持ち主 owner から完全に削除された to への関連を外した通知。to の記録か持ち主がなければ false。
func relationNotice(r port.Reader, b batch, owner change.Ref, to item.ID) (port.Notice, bool, error) {
	cs, err := b.causes(r)
	if err != nil {
		return port.Notice{}, false, err
	}
	c, ok := cs.of(to)
	if !ok || c.purged == nil {
		return port.Notice{}, false, nil
	}
	o, ok, err := entityOf(r, b, owner)
	if !ok {
		return port.Notice{}, false, err
	}
	n := o.base
	n.Text, n.Source = purgedText(c.purged.Display), "repair:relation:"+string(owner.ID)+":"+string(to)
	return n, true, nil
}

// relationAdder は、log で関連 rel を最後に足した人(実行者が人の add relations の行のうち、at が最もあとの行の
// by。同じなら ID の大きい行)。なければ空。
func relationAdder(log []change.Record, rel item.Relation) string {
	var w *change.Record
	for j := range log {
		c := &log[j]
		var got item.Relation
		if c.Actor.Kind != change.ActorUser || c.T != change.Add || c.F != "relations" ||
			json.Unmarshal(c.To, &got) != nil || got != rel {
			continue
		}
		if w == nil || c.At.After(w.At) || (c.At.Equal(w.At) && c.ID > w.ID) {
			w = c
		}
	}
	if w == nil {
		return ""
	}
	return w.By
}

// droppedEntities は、i 番目が合わせなら、合わせで消えたエンティティのうち、共通の祖先にない行(New)にあなたの
// 行があるものごとの通知(06「完全な削除と重なったとき」「別の Store へ移動したものと重なったとき」。C4b2-7・
// C4b2-12)。消える前のログ(Log)だけにある行は数えない。
func droppedEntities(r port.Reader, b batch, i int, me string) ([]port.Notice, error) {
	s := b.saves[i]
	if !s.Merge {
		return nil, nil
	}
	var out []port.Notice
	for _, e := range s.Removed {
		if !wroteBy(e.New, me) {
			continue
		}
		n, ok, err := droppedNotice(r, b, s.Removed, e)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, n)
		}
	}
	return out, nil
}

// wroteBy は、rows に、実行者が人で by が me の行があるか。
func wroteBy(rows []change.Record, me string) bool {
	for _, row := range rows {
		if row.Actor.Kind == change.ActorUser && same(row.By, me) {
			return true
		}
	}
	return false
}

// droppedNotice は、合わせで消えた e の通知。原因が見つからないか、e を読めなければ false。
func droppedNotice(r port.Reader, b batch, removed []port.RemovedEntity, e port.RemovedEntity) (port.Notice, bool, error) {
	c, ok, err := findCause(r, b, removed, e.Ref)
	if !ok {
		return port.Notice{}, false, err
	}
	g, ok, err := goneOf(r, b, e.Ref, e.Log)
	if !ok {
		return port.Notice{}, false, err
	}
	n := g.base
	if c.purged != nil {
		n.Text, n.Source = purgedText(c.text), "repair:purged:"+string(e.Ref.ID)
		return n, true, nil
	}
	who, err := nameOf(r, c.moved.By)
	if err != nil {
		return port.Notice{}, false, err
	}
	n.Text = withParticle(c.text, "は") + " " + who + " が " + withParticle(oneLine(b.storeName(*c.moved)), "へ") +
		"移動したため、あなたの変更は反映されませんでした"
	if c.at.Kind != change.KindWorkspace {
		n.Text += "。移動先は " + withParticle(c.moved.New, "です")
	}
	if c.at != e.Ref {
		n.Text += "(" + g.display + "「" + oneLine(g.title) + "」)"
	}
	n.Source = "repair:moved:" + string(e.Ref.ID)
	return n, true, nil
}

// purgedText は、完全な削除のために反映されなかった本文。
func purgedText(display string) string {
	return withParticle(display, "は") + "完全に削除されたため、あなたの変更は反映されませんでした"
}

// gone は、消えたものを指す通知の下地と、子の文の {子の ID}・{タイトル}(C4b2-11・C4b2-12)。
type gone struct {
	base           port.Notice
	display, title string
}

// goneOf は、消える前のログ log の ref の下地。アイテム・プロジェクトは消える前の ID(接頭辞はいまの
// ワークスペースのもの、なければ内部 ID)、ワークスペースはその内部 ID だけを持つ。作れなければ false。
func goneOf(r port.Reader, b batch, ref change.Ref, log []change.Record) (gone, bool, error) {
	switch ref.Kind {
	case change.KindItem:
		it, ok := item.FromLog(log)
		if !ok {
			return gone{}, false, nil
		}
		d, err := displayIn(r, b, it.Place.Workspace, string(it.ID), it.DisplayID)
		return gone{port.Notice{Item: it.ID, Display: d}, d, it.Title}, err == nil, err
	case change.KindProject:
		p, ok := project.FromLog(log)
		if !ok {
			return gone{}, false, nil
		}
		d, err := displayIn(r, b, p.Place.Workspace, string(p.ID), p.DisplayID)
		return gone{port.Notice{Project: p.ID, Display: d}, d, p.Title}, err == nil, err
	case change.KindWorkspace:
		w, ok := workspace.FromLog(log)
		return gone{base: port.Notice{Workspace: w.ID}, title: w.Name}, ok, nil
	}
	return gone{}, false, nil
}

// displayIn は、ワークスペース ws のいまの接頭辞で作る ID。ワークスペースがなければ内部 ID id。
func displayIn(r port.Reader, b batch, ws workspace.ID, id string, display func(prefix string) string) (string, error) {
	w, ok, err := workspaceOf(r, b, ws)
	if err != nil {
		return "", err
	}
	if !ok {
		return id, nil
	}
	return display(w.Prefix), nil
}
