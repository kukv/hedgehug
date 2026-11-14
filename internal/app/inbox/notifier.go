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

func (n Notifier) Add(ctx context.Context, notice port.Notice) error {
	if err := n.Inbox.Add(ctx, notice); err != nil {
		return err
	}
	n.Events.Publish(port.Event{Kind: port.EventInbox, Store: notice.Store})
	return nil
}
