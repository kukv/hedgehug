// Package automations は、自動化のルールの操作(仕様 03「自動化」)を持つ。
package automations

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Service は、自動化の操作が使う口。
type Service struct {
	Stores   port.Stores
	IDs      port.IDs
	Identity port.Identity
	Clock    port.Clock
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// rulesOf は、ワークスペースのルール。target(項目名)があれば、それを覆う削除の行がないものとして読む。空なら、いまのルール。
func rulesOf(r port.Reader, ws workspace.ID, target string) ([]automation.Rule, error) {
	if _, err := port.LiveWorkspace(r, ws); err != nil {
		return nil, err
	}
	return rulesIn(r, ws, target)
}

// rulesToSave は、保存の前の読みのルール。revive なら、削除済みのワークスペースのルールも読む(保存で削除を取り消す。決定 R11)。
func rulesToSave(r port.Reader, ws workspace.ID, revive bool, target string) ([]automation.Rule, error) {
	if !revive {
		return rulesOf(r, ws, target)
	}
	if _, err := port.EditableWorkspace(r, ws); err != nil {
		return nil, err
	}
	return rulesIn(r, ws, target)
}

// rulesIn は rulesOf と同じで、ワークスペースが削除済みかを見ない。
func rulesIn(r port.Reader, ws workspace.ID, target string) ([]automation.Rule, error) {
	log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws})
	if err != nil {
		return nil, err
	}
	if target != "" {
		log = workspace.WithoutCovering(log, target)
	}
	return automation.FromLog(log), nil
}

func logins(r port.Reader) ([]string, error) {
	ms, err := r.Members().List()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.Login)
	}
	return out, nil
}

// edit は、summary で決めた文で 1 回保存する。drafts は保存の中のいまのルールから行を作る。どちらも読む口を受ける。
// target(ルールの項目名の頭 rules/<id>)があれば、別の画面で削除されたそのルールへの保存として、削除を取り消して
// 変更を残す(06「他の書き手との重なり」)。保存の前の読みは、取り消す削除の行がないものとして作ったルール(決定 R10)。
// 取り消したのに行がなければ ErrNotFound(決定 R3)、取り消したルールの名前が重なれば断る(決定 R4)。空なら取り消さない。
// ワークスペースが別の画面で削除されていれば、その削除も取り消す(決定 R11)。
func (s Service) edit(ctx context.Context, store port.StoreID, ws workspace.ID, target string,
	summary func(r port.Reader, rules []automation.Rule) (string, error),
	drafts func(r port.Reader, st workspace.Settings, set projectrule.Set, rules []automation.Rule, members []string) ([]change.Draft, error),
) error {
	return s.save(ctx, store, ws, true, target, summary, drafts)
}

// remove は edit と同じで、ルールの削除が使う。削除済みのワークスペースは取り消さずに ErrNotFound(決定 R11)。
func (s Service) remove(ctx context.Context, store port.StoreID, ws workspace.ID,
	summary func(r port.Reader, rules []automation.Rule) (string, error),
	drafts func(r port.Reader, st workspace.Settings, set projectrule.Set, rules []automation.Rule, members []string) ([]change.Draft, error),
) error {
	return s.save(ctx, store, ws, false, "", summary, drafts)
}

// save は、edit・remove の中身。revive が真なら、削除済みのワークスペースの削除を取り消す。
func (s Service) save(ctx context.Context, store port.StoreID, ws workspace.ID, revive bool, target string,
	summary func(r port.Reader, rules []automation.Rule) (string, error),
	drafts func(r port.Reader, st workspace.Settings, set projectrule.Set, rules []automation.Rule, members []string) ([]change.Draft, error),
) error {
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	var text string
	err = s.Stores.Read(ctx, store, func(r port.Reader) error {
		rules, err := rulesToSave(r, ws, revive, target)
		if err != nil {
			return err
		}
		text, err = summary(r, rules)
		return err
	})
	if err != nil {
		return err
	}
	return s.Stores.Write(ctx, store, port.Operation{Actor: change.User(me), Summary: text}, func(tx port.Tx) error {
		return writeRules(ctx, tx, ws, revive, target, drafts)
	})
}

// reviveFor は、書く Tx の中で、revive ならワークスペースの削除を、target があればそのルールの削除を取り消す。
// それぞれ取り消したら true。
func reviveFor(ctx context.Context, tx port.Tx, ws workspace.ID, revive bool, target string) (wsRevived, ruleRevived bool, err error) {
	if revive {
		if wsRevived, err = port.ReviveWorkspace(ctx, tx, ws, "", ""); err != nil {
			return false, false, err
		}
	}
	if target != "" {
		if ruleRevived, err = port.ReviveSetting(ctx, tx, ws, target); err != nil {
			return false, false, err
		}
	}
	return wsRevived, ruleRevived, nil
}

// writeRules は、save の書く Tx の中身。ワークスペース(revive のとき)と target の削除を取り消し、読み直したルールから
// drafts で行を書く。
func writeRules(ctx context.Context, tx port.Tx, ws workspace.ID, revive bool, target string,
	drafts func(r port.Reader, st workspace.Settings, set projectrule.Set, rules []automation.Rule, members []string) ([]change.Draft, error),
) error {
	wsRevived, revived, err := reviveFor(ctx, tx, ws, revive, target)
	if err != nil {
		return err
	}
	rules, err := rulesOf(tx, ws, "")
	if err != nil {
		return err
	}
	st, err := tx.Workspaces().Settings(ws)
	if err != nil {
		return err
	}
	ms, err := logins(tx)
	if err != nil {
		return err
	}
	log, err := tx.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws})
	if err != nil {
		return err
	}
	ds, err := drafts(tx, st, projectrule.FromLog(log), rules, ms)
	if err != nil {
		return err
	}
	if (wsRevived || revived) && len(ds) == 0 {
		return port.ErrNotFound
	}
	if err := tx.WriteWorkspaces().Append(ws, ds...); err != nil {
		return err
	}
	if revived {
		return checkRevivedName(tx, ws, target)
	}
	return nil
}

// checkRevivedName は、行を書いたあとのルールで、削除を取り消した target のルールの名前が、ほかのルールと重ならないか
// 確かめる(決定 R4)。
func checkRevivedName(tx port.Tx, ws workspace.ID, target string) error {
	rules, err := rulesOf(tx, ws, "")
	if err != nil {
		return err
	}
	for _, r := range rules {
		if ruleField(r.ID) == target {
			return automation.Check(rules, r)
		}
	}
	return nil
}

func find(rules []automation.Rule, id change.ID) (automation.Rule, error) {
	for _, r := range rules {
		if r.ID == id {
			return r, nil
		}
	}
	return automation.Rule{}, port.ErrNotFound
}

// canonicalLogin は、担当者を入れるルールの値を、メンバーの保存どおりの綴りにする。
// Valid は大文字小文字を区別しないが、実行は区別して比べるため。
func canonicalLogin(r automation.Rule, ms []string) automation.Rule {
	if r.Action != automation.ActionAssignee || r.Value == automation.Unassigned {
		return r
	}
	for _, l := range ms {
		if strings.EqualFold(l, r.Value) {
			r.Value = l
			break
		}
	}
	return r
}
