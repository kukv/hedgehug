package worklogs

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/worklog"
)

// Delete は、作業時間の記録を削除する(to を null にした行を足す)。誰の記録でも、書き込める人なら消せる(02「作業時間」)。
// アクティビティは「{ID} の作業時間を削除」。ないか、すでに削除された記録は port.ErrNotFound。
func (s Service) Delete(ctx context.Context, store port.StoreID, id item.ID, w change.ID) error {
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	shown, err := s.displayOf(ctx, store, id)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: change.User(me), Summary: shown + " の作業時間を削除"}
	return s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		if err := revive(ctx, tx, id); err != nil {
			return err
		}
		log, err := tx.Changes().Of(change.Ref{Kind: change.KindItem, ID: id})
		if err != nil {
			return err
		}
		e, ok := worklog.Find(log, w)
		if !ok {
			return port.ErrNotFound
		}
		return tx.WriteItems().Append(id, change.NewWorklog(w, worklog.Value(e.Hundredths, e.Memo, e.Date), nil))
	})
}
