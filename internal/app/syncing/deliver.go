package syncing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// Deliver は、Store id に fn で保存し、remote があれば同じロックの中で受け取って送る(02「保存」の (1)〜(3))。
// 送れなければ保存しない。理由が空なら保存できた。失敗の理由は syncstatus.Reason で返し、同期の状態に記録する
// (アカウントがないときは記録しない)。error は、理由で表せない失敗だけ: Store がない(port.ErrStoreNotFound)、
// 閲覧のみ(port.ErrInvalidOperation)、fn・同期の修復などの保存の失敗(中のエラーをそのまま)、このマシンの状態を読み書き
// できない、送れなかった保存を捨てられなかった(port.ErrSaveNotDiscarded)。呼ぶ側は、ErrSaveNotDiscarded の
// error を「保存されたかもしれない」(このマシンに残り、次の同期で送られうる)と扱う。それ以外の error と理由は
// 「保存されていない」。保存して送れたあとに状態の記録やミラーの記録が失敗しても、ログに残すだけで成功を返す
// (保存は送られている。状態は次の同期で直る)。remote があれば fn は送信が重なるたびに呼ばれ直す(port.Replicator.SyncSave)。同期の始めた要求が切れても
// 最後まで続けるのと同じく(決定 C2-17)、ctx が切れても続ける。AfterSuccess は呼ばない。
func (s *Service) Deliver(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) (syncstatus.Reason, error) {
	ctx = context.WithoutCancel(ctx)
	targets, err := s.Targets.List(ctx)
	if err != nil {
		return "", err
	}
	for _, t := range targets {
		if t.ID != id {
			continue
		}
		switch {
		case t.ReadOnly:
			return "", fmt.Errorf("%w: store %s is read-only", port.ErrInvalidOperation, id)
		case t.Remote == "":
			return "", s.Stores.Write(ctx, id, op, fn)
		}
		return s.deliverRemote(ctx, t, op, fn)
	}
	return "", port.ErrStoreNotFound
}

// deliverRemote は、remote のある Store t に、トークン・受け取った位置・同期の状態の記録を run と同じに扱って
// SyncSave で保存して送り、成功したらミラーへ送って知らせる。
func (s *Service) deliverRemote(ctx context.Context, t port.SyncTarget, op port.Operation, fn func(tx port.Tx) error) (syncstatus.Reason, error) {
	out := Outcome{Store: t.ID, Name: t.Name}
	at := s.Clock.Now()
	tok, r, err := s.token(ctx, t)
	switch {
	case err != nil:
		return "", err
	case r == syncstatus.NoAccount:
		return r, nil // 記録しない。状態は Status が導く(決定 C2-16)
	case r != "":
		out, err = s.failed(ctx, out, at, r)
		return out.Reason, err
	}
	key := remoteKey(t.Remote)
	seen, err := s.State.Seen(ctx, t.ID, key)
	if err != nil {
		return "", err
	}
	res, err := s.Replicator.SyncSave(ctx, t.ID, port.Remote{URL: t.Remote, Token: tok, Seen: seen}, op, fn)
	if se := (*port.SaveError)(nil); errors.As(err, &se) {
		return "", se.Err
	}
	if errors.Is(err, port.ErrSaveNotDiscarded) {
		return "", err // 理由にしない(理由は「保存されていない」と読まれる)
	}
	if err != nil {
		r := remoteReason(t.ID, err)
		if r == syncstatus.AuthFailed {
			s.Tokens.StoreRejected(ctx, t.ID)
		}
		out, err = s.failed(ctx, out, at, r)
		return out.Reason, err
	}
	// ここからは保存が送られている。記録が失敗しても成功を返す(呼ぶ側が「保存されていない」と読まないため)。
	if err := s.State.Succeeded(ctx, t.ID, at, key, res.Seen); err != nil {
		slog.Warn("送ったあとに同期の状態を記録できませんでした", "store", t.ID, "error", err)
	}
	if _, err := s.mirror(ctx, t, tok); err != nil {
		slog.Warn("送ったあとにミラーの状態を記録できませんでした", "store", t.ID, "error", err)
	}
	s.publish(t.ID)
	return "", nil
}
