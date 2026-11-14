package noticing

import (
	"context"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// automations は、判定する保存のうち、ステータスの削除でルールが変わったものから、変わったルール 1 つにつき 1 件の
// 通知(C4b1-14)。文は削除した人のマシンの通知(workspaces の notifyRules)と同じ。削除した人があなたでも作る。
// 別の Store から移ってきたワークスペースは、写した古い行なので作らない(C4b1-19 と同じ理由)。
func automations(b batch, store port.StoreID) []port.Notice {
	var out []port.Notice
	for i, s := range b.saves {
		for _, e := range s.Entities {
			if e.Ref.Kind != change.KindWorkspace || b.movedIn(e.Ref) {
				continue
			}
			for _, n := range ruleNotices(b, i, e) {
				n.Store, n.Workspace = store, e.Ref.ID
				out = append(out, n)
			}
		}
	}
	return out
}

// ruleNotices は、i 番目の保存がワークスペースのログに足した行 e のうち、ステータスの削除で変わったルールの通知。
// ルールは保存の後、削除したステータスの名前は保存の前の設定で引く。
func ruleNotices(b batch, i int, e port.EntityRows) []port.Notice {
	if _, ok := b.logs[e.Ref]; !ok {
		return nil
	}
	deleted, changes := automation.FollowedFromRows(e.Rows, automation.FromLog(b.until(e.Ref, i+1)))
	if deleted == "" {
		return nil
	}
	st, ok := workspace.SettingsFromLog(b.until(e.Ref, i))
	if !ok {
		return nil
	}
	var out []port.Notice
	for _, c := range changes {
		if n, ok := ruleNotice(st, deleted, c); ok {
			out = append(out, n)
		}
	}
	return out
}

// ruleNotice は、変わったルール c の通知。ステータスの名前を引けなければ false(C4b1-5)。
func ruleNotice(st workspace.Settings, deleted change.ID, c automation.RuleChange) (port.Notice, bool) {
	n := port.Notice{Kind: port.NoticeAutomation, Rule: c.Rule.ID}
	if c.Disabled {
		n.Text = automation.DisabledByDeleteText(c.Rule.Name)
		return n, true
	}
	to := c.Rule.TriggerStatus
	if c.Part == automation.PartAction {
		to = change.ID(c.Rule.Value)
	}
	from, okFrom := statusName(st, deleted)
	repl, okTo := statusName(st, to)
	if !okFrom || !okTo {
		return port.Notice{}, false
	}
	n.Text = automation.RewrittenText(c.Rule.Name, from, c.Part, repl)
	return n, true
}

func statusName(st workspace.Settings, id change.ID) (string, bool) {
	i := slices.IndexFunc(st.Statuses, func(v workspace.Value) bool { return v.ID == id })
	if i < 0 {
		return "", false
	}
	return st.Statuses[i].Name, true
}

// until は、ref のいまのログから、saves[n:] が足した行を除いたもの(n 番目の保存の前の状態)。
func (b batch) until(ref change.Ref, n int) []change.Record {
	later := map[change.ID]bool{}
	for _, s := range b.saves[n:] {
		for _, e := range s.Entities {
			if e.Ref != ref {
				continue
			}
			for _, r := range e.Rows {
				later[r.ID] = true
			}
		}
	}
	return slices.DeleteFunc(slices.Clone(b.logs[ref]), func(r change.Record) bool { return later[r.ID] })
}

// writable は、Store が表示中で閲覧のみでないか。非表示の Store は一覧にない(06「自動化」)。
func (s Service) writable(ctx context.Context, store port.StoreID) (bool, error) {
	infos, err := s.List.List(ctx)
	if err != nil {
		return false, err
	}
	i := slices.IndexFunc(infos, func(info port.StoreInfo) bool { return info.ID == store })
	return i >= 0 && !infos[i].ReadOnly, nil
}
