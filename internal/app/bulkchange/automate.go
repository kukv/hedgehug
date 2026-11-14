package bulkchange

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// fired は、1 つのアイテムで動いたルールの結果と、通知の文を作るために保存の中で読んだもの
// (app/githublink の fired と同じ形。app の機能どうしは import しない)。
type fired struct {
	item     item.Item
	outcomes []automation.Outcome
	ws       *wsChoices
	names    item.Names
}

// parentRun は、ステータスを変えた Step の親と、変えた Step の操作の前のステータス。
type parentRun struct {
	id     item.ID
	ws     *wsChoices
	before map[item.ID]change.ID
}

// automate は、利用者の行を全部書いたあとに、同じ保存の中で自動化を動かす(F2b2-14)。「Step がすべて完了した
// とき」の判定は、ステータスのきっかけのルールより先に、利用者の値で行う(自動化の変更を数えない。B4-8)。
// そのあと、ステータスを変えたアイテムごとに選んだ順で「ステータスが次になったとき」を、合った親ごとに
// 最初に出てきた順で 1 回「Step がすべて完了したとき」を動かす。削除済みの親には動かさない。
func automate(tx port.Tx, js []judged) ([]fired, error) {
	parents, err := stepsDoneParents(tx, js)
	if err != nil {
		return nil, err
	}
	names, err := memberNames(tx)
	if err != nil {
		return nil, err
	}
	out, err := runStatus(tx, js, names)
	if err != nil {
		return nil, err
	}
	for _, p := range parents {
		parent, err := tx.Items().Get(p.id)
		if err != nil {
			return nil, err
		}
		if parent.Deleted != nil {
			continue // 削除済みのアイテムには動かない(03「実行の規則」)
		}
		if out, err = runOn(tx, out, p.ws, names, parent, automation.Event{Trigger: automation.TriggerStepsDone}); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// runStatus は、ステータスを変えたアイテムごとに選んだ順で「ステータスが次になったとき」を動かす。
func runStatus(tx port.Tx, js []judged, names item.Names) ([]fired, error) {
	var out []fired
	for _, j := range js {
		if !j.changes() {
			continue
		}
		it, err := port.LiveItem(tx, j.t.ID)
		if err != nil {
			return nil, err
		}
		if out, err = runOn(tx, out, j.ws, names, it, automation.Event{Trigger: automation.TriggerStatus, Status: it.Status}); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// stepsDoneParents は、ステータスを変えた Step の親のうち、この操作で Step がすべて完了・中止になったもの。
// 前の状態は、変えた Step の前のステータスをすべて差し替えたもの。
func stepsDoneParents(tx port.Tx, js []judged) ([]parentRun, error) {
	var ps []parentRun
	for _, j := range js {
		it := j.c.it
		if !j.changes() || it.Kind != item.Step || it.Place.Parent == nil {
			continue
		}
		i := 0
		for i < len(ps) && ps[i].id != *it.Place.Parent {
			i++
		}
		if i == len(ps) {
			ps = append(ps, parentRun{id: *it.Place.Parent, ws: j.ws, before: map[item.ID]change.ID{}})
		}
		ps[i].before[it.ID] = it.Status
	}
	var out []parentRun
	for _, p := range ps {
		steps, err := stepsOf(tx, p)
		if err != nil {
			return nil, err
		}
		if automation.StepsDone(p.ws.settings, steps, p.before) {
			out = append(out, p)
		}
	}
	return out, nil
}

// stepsOf は、親 p の Step(いまの値。利用者の行を書いたあと)。Step は親と同じプロジェクトにある。
func stepsOf(tx port.Tx, p parentRun) ([]item.Item, error) {
	parent, err := tx.Items().Get(p.id)
	if err != nil {
		return nil, err
	}
	all, err := port.LiveItems(tx, parent.Place.Project)
	if err != nil {
		return nil, err
	}
	var out []item.Item
	for _, x := range all {
		if x.Kind == item.Step && x.Place.Parent != nil && *x.Place.Parent == p.id {
			out = append(out, x)
		}
	}
	return out, nil
}

// runOn は、it でルールを動かし、変えた行を書いて、結果を out に足す。
func runOn(tx port.Tx, out []fired, w *wsChoices, names item.Names, it item.Item, ev automation.Event) ([]fired, error) {
	os := automation.Run(w.settings, w.rules, w.autos, it, ev)
	if len(os) == 0 {
		return out, nil
	}
	var ds []change.Draft
	for _, o := range os {
		if o.Draft != nil {
			ds = append(ds, *o.Draft)
		}
	}
	return append(out, fired{item: it, outcomes: os, ws: w, names: names}), tx.WriteItems().Append(it.ID, ds...)
}

// memberNames は、メンバーの表示名を引く関数(app/githublink の memberNames と同じ)。
func memberNames(r port.Reader) (item.Names, error) {
	ms, err := r.Members().List()
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	for _, m := range ms {
		names[strings.ToLower(m.Login)] = m.Name
	}
	return func(login string) string {
		if n, ok := names[strings.ToLower(login)]; ok {
			return n
		}
		return "不明なユーザー" // 01「画面の移り方と、共通の表示」
	}, nil
}

// notify は、保存のあとに、動いたルールの通知を受信箱に書く(app/githublink の notify と同じ形)。
func (s Service) notify(ctx context.Context, store port.StoreID, fs []fired) error {
	for _, f := range fs {
		for _, o := range f.outcomes {
			n := port.Notice{Store: store, Workspace: f.item.Place.Workspace, Item: f.item.ID, Rule: o.Rule.ID, Display: f.item.DisplayID(f.ws.prefix)}
			if o.Draft == nil {
				n.Kind, n.Text, n.Reason = port.NoticeAutomationStopped, automation.StoppedText(o.Rule.Name), o.Stopped
				n.Status, n.From = change.ID(o.Rule.Value), o.From
			} else {
				value := item.Display(o.Prop, o.To, f.ws.settings, f.names, s.Clock.Now())
				n.Kind, n.Text = port.NoticeAutomation, automation.AppliedText(o.Rule.Name, automation.ChangeText(o.Prop.Name, value))
			}
			if err := s.Inbox.Add(ctx, n); err != nil {
				return err
			}
		}
	}
	return nil
}
