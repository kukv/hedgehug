package comments

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/comment"
)

// Delete は、コメントを削除する(to を null にした行を足す)。アクティビティは「コメントを削除」。
// ないか、すでに削除されたコメント、投稿者が「あなた」でないコメントは port.ErrNotFound
// (画面からは起きない操作。ログイン名は大文字と小文字を区別しない)。
func (s Service) Delete(ctx context.Context, store port.StoreID, t Target, id change.ID) error {
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: change.User(me), Summary: "コメントを削除"}
	return s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		cur, err := currentOf(tx, t, id)
		if err != nil {
			return err
		}
		if !strings.EqualFold(cur.By, me) {
			return port.ErrNotFound
		}
		return appendTo(tx, t, change.NewComment(id, comment.Value(cur.Body), nil))
	})
}
