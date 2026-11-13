package items

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// fired は、1 つのアイテムで動いたルールの結果。
type fired struct {
	item     item.Item
	outcomes []automation.Outcome
}

// automate は、利用者がアイテム id のステータスを from から変えたあとに、同じ保存の中で自動化を動かす
// (03「実行の規則」)。止まった実行は何も書かない。結果は保存のあとの通知に使う。
func automate(tx port.Tx, st workspace.Settings, id item.ID, from change.ID) ([]fired, error) {
	it, err := port.LiveItem(tx, id)
	if err != nil {
		return nil, err
	}
	log, err := tx.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: it.Place.Workspace})
	if err != nil {
		return nil, err
	}
	rules, set := automation.FromLog(log), projectrule.FromLog(log)
	out, err := runOn(tx, st, set, rules, it, automation.Event{Trigger: automation.TriggerStatus, Status: it.Status})
	if err != nil || it.Kind != item.Step || it.Place.Parent == nil {
		return out, err
	}
	steps, err := stepsOf(tx, it)
	if err != nil || !automation.StepsDone(st, steps, map[item.ID]change.ID{it.ID: from}) {
		return out, err
	}
	parent, err := tx.Items().Get(*it.Place.Parent)
	if err != nil {
		return nil, err
	}
	if parent.Deleted != nil {
		return out, nil // 削除済みのアイテムには動かない(03「実行の規則」)
	}
	more, err := runOn(tx, st, set, rules, parent, automation.Event{Trigger: automation.TriggerStepsDone})
	return append(out, more...), err
}

// runOn は、it でルールを動かし、変えた行を書く。
func runOn(tx port.Tx, st workspace.Settings, set projectrule.Set, rules []automation.Rule, it item.Item, ev automation.Event) ([]fired, error) {
	os := automation.Run(st, set, rules, it, ev)
	if len(os) == 0 {
		return nil, nil
	}
	var ds []change.Draft
	for _, o := range os {
		if o.Draft != nil {
			ds = append(ds, *o.Draft)
		}
	}
	return []fired{{item: it, outcomes: os}}, tx.WriteItems().Append(it.ID, ds...)
}

// stepsOf は、it と同じ親の Step。it は利用者が変えた値で数える(自動化の変更を入れない。B4-8)。
func stepsOf(tx port.Tx, it item.Item) ([]item.Item, error) {
	all, err := port.LiveItems(tx, it.Place.Project)
	if err != nil {
		return nil, err
	}
	var out []item.Item
	for _, x := range all {
		if x.Kind != item.Step || x.Place.Parent == nil || *x.Place.Parent != *it.Place.Parent {
			continue
		}
		if x.ID == it.ID {
			x = it
		}
		out = append(out, x)
	}
	return out, nil
}

// notify は、保存のあとに、動いたルールの通知を書く(B4-14)。
func (s Service) notify(ctx context.Context, store port.StoreID, ec editContext, fs []fired) error {
	for _, f := range fs {
		for _, o := range f.outcomes {
			n := port.Notice{Store: store, Workspace: f.item.Place.Workspace, Item: f.item.ID, Rule: o.Rule.ID}
			if o.Draft == nil {
				n.Kind, n.Text, n.Reason = port.NoticeAutomationStopped, automation.StoppedText(o.Rule.Name), o.Stopped
				n.Status = change.ID(o.Rule.Value)
			} else {
				value := item.Display(o.Prop, o.To, ec.settings, ec.names, s.Clock.Now())
				n.Kind, n.Text = port.NoticeAutomation, automation.AppliedText(o.Rule.Name, automation.ChangeText(o.Prop.Name, value))
			}
			if err := s.Inbox.Add(ctx, n); err != nil {
				return err
			}
		}
	}
	return nil
}
