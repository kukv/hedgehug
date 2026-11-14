package inbox

import (
	"context"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// Revert は、上書きの通知の「自分の値に戻す」。いまの値が残った値(Kept)のままのときだけ、Mine に戻す 1 回の保存を書く。
// 保存できたら、既読にして結果を applied にする(C4c2-9)。ワークフローの遷移・プロジェクトが使わないステータスは検査せず、
// 自動化も動かさない(C4c2-7)。
func (s Service) Revert(ctx context.Context, id port.NoticeID) error {
	n, err := s.pending(ctx, id, port.NoticeOverwrite)
	if err != nil {
		return err
	}
	me, err := s.Identity.Me(ctx, n.Store)
	if err != nil {
		return err
	}
	today := s.Clock.Now()
	var summary string
	err = s.Stores.Read(ctx, n.Store, func(r port.Reader) error {
		sl, state, err := slotOf(r, n, today)
		if err != nil {
			return err
		}
		if state == slotNone {
			return port.ErrInvalidOperation
		}
		summary = sl.summary
		return nil
	})
	if err != nil {
		return err
	}
	op := port.Operation{Actor: change.User(me), Summary: summary}
	err = s.Stores.Write(ctx, n.Store, op, func(tx port.Tx) error { return revertSlot(tx, n, today) })
	if err != nil {
		return err
	}
	if err := s.Book.Resolve(ctx, id, port.NoticeApplied); err != nil {
		return err
	}
	s.publish(n.Store)
	return nil
}

// revertSlot は、保存の中で戻す相手を引き直し、断る条件(C4c2-9)に当たらなければ戻す行を書く。行がなければ何も書かない。
func revertSlot(tx port.Tx, n port.NoticeRow, today time.Time) error {
	sl, state, err := slotOf(tx, n, today)
	if err != nil {
		return err
	}
	if state != slotLive || !sameValue(sl.now, n.Kept) {
		display := sl.display
		if display == "" {
			display = n.Display
		}
		return refusal.New(display + " は先に変更されたため、適用しませんでした")
	}
	if sl.reason != "" {
		return refusal.New(sl.reason)
	}
	ds, err := sl.drafts(tx)
	if err != nil || len(ds) == 0 {
		return err
	}
	return sl.write(tx, ds)
}
