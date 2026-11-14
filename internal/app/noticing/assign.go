package noticing

import (
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// assigns は、判定の中で担当者があなたになったアイテムと、オーナーがあなたになったプロジェクトの通知
// (06「割り当て」)。エンティティごとに、判定の前と後を比べる。
func assigns(r port.Reader, b batch, store port.StoreID, me string) []port.Notice {
	var out []port.Notice
	for _, ref := range b.refs {
		if b.movedIn(ref) {
			continue
		}
		var n port.Notice
		var ok bool
		switch ref.Kind {
		case change.KindItem:
			n, ok = assignItem(r, b, ref, me)
		case change.KindProject:
			n, ok = ownerProject(r, b, ref, me)
		}
		if ok {
			n.Kind, n.Store = port.NoticeAssign, store
			out = append(out, n)
		}
	}
	return out
}

func assignItem(r port.Reader, b batch, ref change.Ref, me string) (port.Notice, bool) {
	after, n, ok := itemTarget(r, b, ref)
	if !ok || !same(after.Assignee, me) {
		return port.Notice{}, false
	}
	if before, ok := item.FromLog(b.before(ref)); ok && same(before.Assignee, me) {
		return port.Notice{}, false
	}
	w, ok := byOther(b.after(ref), "assignee", me)
	if !ok {
		return port.Notice{}, false
	}
	n.Text, n.Sender = "あなたに割り当てました", sender(w)
	return n, true
}

func ownerProject(r port.Reader, b batch, ref change.Ref, me string) (port.Notice, bool) {
	after, n, ok := projectTarget(r, b, ref)
	if !ok || !same(after.Owner, me) {
		return port.Notice{}, false
	}
	if before, ok := project.FromLog(b.before(ref)); ok && same(before.Owner, me) {
		return port.Notice{}, false
	}
	w, ok := byOther(b.after(ref), "owner", me)
	if !ok {
		return port.Notice{}, false
	}
	n.Text, n.Sender = "プロジェクト「"+after.Title+"」のオーナーになりました", sender(w)
	return n, true
}

// byOther は、項目 f のいまの値を決めた行(set がなければ作成の行)を返す。その行があなたの変更、GitHub の
// 操作、同期の修復なら false(C4b1-7)。
func byOther(log []change.Record, f, me string) (change.Record, bool) {
	w, ok := change.Current(log, change.Set, f, func(change.Record) bool { return true })
	if !ok {
		c := change.Resolve(log, change.Options{}).Created
		if c == nil {
			return change.Record{}, false
		}
		w = *c
	}
	if w.Actor.Kind == change.ActorGitHub || w.Actor.Kind == change.ActorSync || same(w.By, me) {
		return change.Record{}, false
	}
	return w, true
}

// itemTarget は、判定の後のアイテムと、それを指す通知の下地(ワークスペース・アイテム・表示用の ID)。
// アイテムかワークスペースがなければ false。
func itemTarget(r port.Reader, b batch, ref change.Ref) (item.Item, port.Notice, bool) {
	it, ok := item.FromLog(b.after(ref))
	if !ok {
		return item.Item{}, port.Notice{}, false
	}
	ws, err := r.Workspaces().Get(it.Place.Workspace)
	if err != nil {
		return item.Item{}, port.Notice{}, false
	}
	return it, port.Notice{Workspace: ws.ID, Item: ref.ID, Display: it.DisplayID(ws.Prefix)}, true
}

// projectTarget は、判定の後のプロジェクトと、それを指す通知の下地(C4b1-12)。
func projectTarget(r port.Reader, b batch, ref change.Ref) (project.Project, port.Notice, bool) {
	p, ok := project.FromLog(b.after(ref))
	if !ok {
		return project.Project{}, port.Notice{}, false
	}
	ws, err := r.Workspaces().Get(p.Place.Workspace)
	if err != nil {
		return project.Project{}, port.Notice{}, false
	}
	return p, port.Notice{Workspace: ws.ID, Project: ref.ID, Display: p.DisplayID(ws.Prefix)}, true
}
