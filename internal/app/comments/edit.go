package comments

import (
	"context"
	"errors"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/comment"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// Edit は、コメントの本文を変える(02「コメント」の編集)。前後の空白を除いたあとが空なら断り、
// 前と同じなら何も保存しない(持ち主が削除済みなら、取り消さずに port.ErrNotFound で断る)。アクティビティは「コメントを編集」。
// 別の画面で削除された自分のコメントは、削除の前と違う本文なら、削除を取り消して本文を変える(Revived に積む)。
// ないコメント、投稿者が「あなた」でないコメントは port.ErrNotFound
// (画面からは起きない操作。ログイン名は大文字と小文字を区別しない)。
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
		if errors.Is(err, port.ErrNotFound) {
			return reviveComment(ctx, tx, t, id, me, body)
		}
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

// reviveComment は、削除されたコメント id を、投稿者の me が新しい本文 body で書き直すとき、削除を取り消す。
// 削除済みのコメントがないか、他人のものか、本文が削除の前と同じなら port.ErrNotFound。
func reviveComment(ctx context.Context, tx port.Tx, t Target, id change.ID, me, body string) error {
	log, err := tx.Changes().Of(t.ref())
	if err != nil {
		return err
	}
	c, ok := comment.FindDeleted(log, id)
	if !ok || !strings.EqualFold(c.By, me) || c.Body == body {
		return port.ErrNotFound
	}
	display, err := editableDisplay(tx, t)
	if err != nil {
		return err
	}
	if err := appendTo(tx, t, change.NewComment(id, nil, comment.Value(body))); err != nil {
		return err
	}
	port.NoteRevived(ctx, port.Revival{Kind: port.RevivedComment, Text: display})
	return nil
}
