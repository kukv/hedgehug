package worklogs

import (
	"context"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/worklog"
)

// Add は、作業時間を記録する(02「作業時間」)。hours は worklog.ParseHours で検査し、断りはその文言。
// 日付は保存したマシンの今日、記録した人は「あなた」。メモは前後の空白を除く。
// アクティビティは「{ID} に N 時間を記録」。記録を返す。
func (s Service) Add(ctx context.Context, store port.StoreID, id item.ID, hours, memo string) (View, error) {
	h, err := worklog.ParseHours(hours)
	if err != nil {
		return View{}, err
	}
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return View{}, err
	}
	shown, err := s.displayOf(ctx, store, id)
	if err != nil {
		return View{}, err
	}
	w := s.IDs.New()
	date := s.Clock.Now().Format("2006-01-02")
	op := port.Operation{Actor: change.User(me), Summary: fmt.Sprintf("%s に %s 時間を記録", shown, worklog.Format(h))}
	err = s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		if err := revive(ctx, tx, id); err != nil {
			return err
		}
		return tx.WriteItems().Append(id, change.NewWorklog(w, nil, worklog.Value(h, strings.TrimSpace(memo), date)))
	})
	if err != nil {
		return View{}, err
	}
	return s.find(ctx, store, id, w)
}
