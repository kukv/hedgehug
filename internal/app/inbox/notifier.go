package inbox

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
)

// Notifier は、通知を足してから画面に知らせる port.Inbox(C4a-13)。
type Notifier struct {
	Inbox  port.Inbox
	Events port.Events
}

// Add は、通知を足してから受信箱の出来事を出す。足せなかったら出さない。
func (n Notifier) Add(ctx context.Context, notice port.Notice) error {
	if err := n.Inbox.Add(ctx, notice); err != nil {
		return err
	}
	n.Events.Publish(port.Event{Kind: port.EventInbox, Store: notice.Store})
	return nil
}

// Mention は、メンションの行を足すか書き換えてから受信箱の出来事を出す。失敗したら出さない。
func (n Notifier) Mention(ctx context.Context, notice port.Notice) error {
	if err := n.Inbox.Mention(ctx, notice); err != nil {
		return err
	}
	n.Events.Publish(port.Event{Kind: port.EventInbox, Store: notice.Store})
	return nil
}

// AddOnce は、同じ鍵の行がなくて足したときだけ受信箱の出来事を出す。重なったときと失敗したときは出さない。
func (n Notifier) AddOnce(ctx context.Context, notice port.Notice) (bool, error) {
	added, err := n.Inbox.AddOnce(ctx, notice)
	if err != nil || !added {
		return false, err
	}
	n.Events.Publish(port.Event{Kind: port.EventInbox, Store: notice.Store})
	return true, nil
}
