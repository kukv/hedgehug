package comments

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/comment"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// Post は、コメントを投稿する(02「コメント」の投稿)。本文の前後の空白を除き、空なら断る。
// アクティビティは「{ID} にコメント」。投稿したコメントを返す。
func (s Service) Post(ctx context.Context, store port.StoreID, t Target, body string) (View, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return View{}, refusal.New(emptyBody)
	}
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return View{}, err
	}
	shown, err := s.displayOf(ctx, store, t)
	if err != nil {
		return View{}, err
	}
	id := s.IDs.New()
	op := port.Operation{Actor: change.User(me), Summary: shown + " にコメント"}
	err = s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		if _, err := reviveOwner(ctx, tx, t); err != nil {
			return err
		}
		return appendTo(tx, t, change.NewComment(id, nil, comment.Value(body)))
	})
	if err != nil {
		return View{}, err
	}
	return s.find(ctx, store, t, id)
}
