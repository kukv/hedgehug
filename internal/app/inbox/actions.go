package inbox

import (
	"context"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// SetRead は、通知を既読・未読にする。
func (s Service) SetRead(ctx context.Context, id port.NoticeID, read bool) error {
	n, err := s.Book.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.Book.SetRead(ctx, id, read); err != nil {
		return err
	}
	s.publish(n.Store)
	return nil
}

// ReadAll は、表示中の Store の、タブ tab の通知をすべて既読にする(C4a-5)。
func (s Service) ReadAll(ctx context.Context, tab Tab) error {
	infos, err := s.visible(ctx)
	if err != nil {
		return err
	}
	if len(infos) > 0 {
		if err := s.Book.ReadAll(ctx, ids(infos), KindsOf(tab)); err != nil {
			return err
		}
	}
	s.publish("")
	return nil
}

// Keep は、止まった通知の「変更しない」。既読にして結果を kept にする。保存はしない(C4a-12)。
func (s Service) Keep(ctx context.Context, id port.NoticeID) error {
	n, err := s.stopped(ctx, id)
	if err != nil {
		return err
	}
	if err := s.Book.Resolve(ctx, id, port.NoticeKept); err != nil {
		return err
	}
	s.publish(n.Store)
	return nil
}

// Apply は、止まった通知の「制約を無視して…」。保存できたら、既読にして結果を applied にする(C4a-11)。
func (s Service) Apply(ctx context.Context, id port.NoticeID) error {
	n, err := s.stopped(ctx, id)
	if err != nil {
		return err
	}
	if err := s.ApplyIgnoringWorkflow(ctx, n.Store, n.Item, n.Rule, n.From, n.Status); err != nil {
		return err
	}
	if err := s.Book.Resolve(ctx, id, port.NoticeApplied); err != nil {
		return err
	}
	s.publish(n.Store)
	return nil
}

// stopped は、ボタンを押せる止まった通知を返す。種類が違えば ErrInvalidOperation、結果があるか閲覧のみの Store なら断る(C4a-10・C4a-12)。
// 表示中でない Store の通知は ErrNotFound(一覧に出ない)。
func (s Service) stopped(ctx context.Context, id port.NoticeID) (port.NoticeRow, error) {
	n, err := s.Book.Get(ctx, id)
	if err != nil {
		return port.NoticeRow{}, err
	}
	if n.Kind != port.NoticeAutomationStopped {
		return port.NoticeRow{}, port.ErrInvalidOperation
	}
	if n.Result != "" {
		return port.NoticeRow{}, refusal.New("この通知はすでに処理されています")
	}
	infos, err := s.List.List(ctx)
	if err != nil {
		return port.NoticeRow{}, err
	}
	k := slices.IndexFunc(infos, func(info port.StoreInfo) bool { return info.ID == n.Store })
	if k < 0 {
		return port.NoticeRow{}, port.ErrNotFound
	}
	if infos[k].ReadOnly {
		return port.NoticeRow{}, refusal.New("閲覧のみの Store です")
	}
	return n, nil
}

// publish は、受信箱が変わったことを画面に知らせる。store が空なら、すべて既読(C4a-13)。
func (s Service) publish(store port.StoreID) {
	s.Events.Publish(port.Event{Kind: port.EventInbox, Store: store})
}
