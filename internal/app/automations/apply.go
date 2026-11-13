package automations

import (
	"bytes"
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// ApplyIgnoringWorkflow は、止まった自動化の通知の「制約を無視して {ステータス名} にする」(03「制約で止まったとき」)。
// 遷移の制約を検査せず、ルールが行った変更(by は押した人)として 1 回で保存する。自動化は動かさない(B4-11)。
func (s Service) ApplyIgnoringWorkflow(ctx context.Context, store port.StoreID, id item.ID, rule, status change.ID) error {
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	name, err := s.ruleName(ctx, store, id, rule)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: change.User(me), Summary: automation.IgnoreText(oneLine(name))}
	return s.Stores.Write(ctx, store, op, func(tx port.Tx) error { return applyStatus(ctx, tx, id, rule, status) })
}

// ruleName は、ワークスペースの変更記録から引いたルール名。消したルールの名前も引ける。
func (s Service) ruleName(ctx context.Context, store port.StoreID, id item.ID, rule change.ID) (string, error) {
	var name string
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		it, err := port.EditableItem(r, id)
		if err != nil {
			return err
		}
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: it.Place.Workspace})
		if err != nil {
			return err
		}
		n, ok := automation.Names(log)[rule]
		if !ok {
			return port.ErrNotFound
		}
		name = n
		return nil
	})
	return name, err
}

// applyStatus は、アイテムのステータスを status にする行を、ルール rule の変更として足す。同じ値なら何も足さない。
// 別の画面で削除された(root: true)アイテムは、行を書くときだけ削除を取り消す(決定 S10)。
func applyStatus(ctx context.Context, tx port.Tx, id item.ID, rule, status change.ID) error {
	it, err := port.EditableItem(tx, id)
	if err != nil {
		return err
	}
	st, err := tx.Workspaces().Settings(it.Place.Workspace)
	if err != nil {
		return err
	}
	p, ok := st.Property("status")
	if !ok {
		return port.ErrNotFound
	}
	to, err := item.ParseInput(p, change.Value(status), st)
	if err != nil {
		return port.ErrNotFound
	}
	from := item.Current(it, p)
	if bytes.Equal(from, to) {
		return port.Unchanged(it.Deleted)
	}
	if err := port.ReviveItem(ctx, tx, id); err != nil {
		return err
	}
	return tx.WriteItems().Append(id, change.NewSet(item.Field(p), from, to).ByRule(rule))
}
