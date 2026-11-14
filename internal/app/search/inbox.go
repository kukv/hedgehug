package search

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// inboxPage は、受信箱を 1 回に読む件数。
const inboxPage = 100

// overwritten は、Store store の、未読の「上書き」の通知があるアイテム(is:overwritten)。受信箱を最後まで読む。
func (s Service) overwritten(ctx context.Context, store port.StoreID) (map[item.ID]bool, error) {
	out := map[item.ID]bool{}
	q := port.NoticeQuery{Stores: []port.StoreID{store}, Kinds: []port.NoticeKind{port.NoticeOverwrite}, Limit: inboxPage}
	for {
		rows, err := s.Inbox.List(ctx, q)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			if !r.Read && r.Item != "" {
				out[r.Item] = true
			}
		}
		if len(rows) < inboxPage {
			return out, nil
		}
		last := rows[len(rows)-1]
		q.Before = &port.NoticeCursor{At: last.At, ID: last.ID}
	}
}
