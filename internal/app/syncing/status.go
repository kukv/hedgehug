package syncing

import (
	"context"
	"log/slog"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// StoreStatus は、remote がある 1 つの Store の同期の状態(06「同期の状態の表示」)。
type StoreStatus struct {
	Store       port.StoreID
	Name        string
	Hidden      bool
	ReadOnly    bool
	Reason      syncstatus.Reason // 同期エラーなら理由
	Unsent      int
	LastSuccess time.Time // 最後に成功した同期。なければゼロ
}

// Status は、remote がある Store の同期の状態を、サイドバーの並びで返す。非表示の Store も含める(画面が
// カードと点から除き、設定の「Store」では出す)。remote があって割り当てがない Store は、試していなくても
// 「アカウントが設定されていません」の同期エラー(決定 C2-16)。
func (s *Service) Status(ctx context.Context) ([]StoreStatus, error) {
	targets, err := s.Targets.List(ctx)
	if err != nil {
		return nil, err
	}
	records, err := s.State.Records(ctx)
	if err != nil {
		return nil, err
	}
	byStore := map[port.StoreID]port.SyncRecord{}
	for _, r := range records {
		byStore[r.Store] = r
	}
	out := []StoreStatus{}
	for _, t := range targets {
		if t.Remote == "" {
			continue
		}
		rec := byStore[t.ID]
		st := StoreStatus{Store: t.ID, Name: t.Name, Hidden: t.Hidden, ReadOnly: t.ReadOnly, Reason: rec.Reason, LastSuccess: rec.LastSuccess}
		if t.Account == "" {
			st.Reason = syncstatus.NoAccount
		}
		// 1 つの Store の数を読めなくても、ほかの Store の状態は返す。その Store は 0 件とする。
		if st.Unsent, err = s.Replicator.Unsent(ctx, t.ID); err != nil {
			slog.Warn("未送信の数を読めませんでした", "store", t.ID, "error", err)
			st.Unsent = 0
		}
		out = append(out, st)
	}
	return out, nil
}
