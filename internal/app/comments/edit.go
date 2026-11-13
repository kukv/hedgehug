package comments

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/comment"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// Edit は、コメントの本文を変える(02「コメント」の編集)。前後の空白を除いたあとが空なら断り、
// 前と同じなら何も保存しない(持ち主が削除済みなら、取り消さずに port.ErrNotFound で断る)。アクティビティは「コメントを編集」。ないか削除されたコメント、
// 投稿者が「あなた」でないコメントは port.ErrNotFound(画面からは起きない操作。ログイン名は大文字と小文字を区別しない)。
func (s Service) Edit(ctx context.Context, store port.StoreID, t Target, id change.ID, body string) (View, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return View{}, refusal.New(emptyBody)
	}
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return View{}, err
	}
	op := port.Operation{Actor: change.User(me), Summary: "コメントを編集"}
	err = s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		deleted, err := reviveOwner(ctx, tx, t)
		if err != nil {
			return err
		}
		cur, err := currentOf(tx, t, id)
		if err != nil {
			return err
		}
		if !strings.EqualFold(cur.By, me) {
			return port.ErrNotFound
		}
		if cur.Body == body {
			return port.Unchanged(deleted) // 行のない Write は何も残さない
		}
		return appendTo(tx, t, change.NewComment(id, comment.Value(cur.Body), comment.Value(body)))
	})
	if err != nil {
		return View{}, err
	}
	return s.find(ctx, store, t, id)
}
