package syncing

import (
	"context"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// run は、1 つの Store の同期の手順(保存形式「同期」)。トークンを得て、前に受け取った位置を読み、
// 受け取り・合わせ・送り、記録し、ミラーへ送る。エラーは、このマシンの状態を読み書きできないときだけ返す。
func (s *Service) run(ctx context.Context, t port.SyncTarget) (Outcome, error) {
	out := Outcome{Store: t.ID, Name: t.Name}
	if t.Remote == "" {
		out.Skipped = true
		return out, nil
	}
	at := s.Clock.Now()
	tok, r, err := s.token(ctx, t)
	switch {
	case err != nil:
		return out, err
	case r == syncstatus.NoAccount:
		out.Reason = r // 記録しない。状態は Status が導く(決定 C2-16)
		return out, nil
	case r != "":
		return s.failed(ctx, out, at, r)
	}
	key := remoteKey(t.Remote)
	seen, err := s.State.Seen(ctx, t.ID, key)
	if err != nil {
		return out, err
	}
	res, err := s.Replicator.Sync(ctx, t.ID, port.Remote{URL: t.Remote, Token: tok, Seen: seen, ReceiveOnly: t.ReadOnly})
	if err != nil {
		r := remoteReason(t.ID, err)
		if r == syncstatus.AuthFailed {
			s.Tokens.StoreRejected(ctx, t.ID)
		}
		return s.failed(ctx, out, at, r)
	}
	if err := s.State.Succeeded(ctx, t.ID, at, key, res.Seen); err != nil {
		return out, err
	}
	out.Sent, out.Received = res.Sent, res.Received
	if !t.ReadOnly {
		if out.MirrorFailures, err = s.mirror(ctx, t, tok); err != nil {
			return out, err
		}
		if s.AfterSuccess != nil && !t.Hidden {
			s.AfterSuccess(ctx, t.ID)
		}
	}
	s.publish(t.ID)
	return out, nil
}

// token は、t の remote に使うトークン。アカウントがなければ理由 syncstatus.NoAccount、得られなければその理由。
// error は、このマシンの失敗(理由で表せない)だけ。
func (s *Service) token(ctx context.Context, t port.SyncTarget) (tok string, r syncstatus.Reason, err error) {
	if t.Account == "" {
		return "", syncstatus.NoAccount, nil
	}
	tok, err = s.Tokens.StoreToken(ctx, t.ID)
	if err != nil {
		r, ok := tokenReason(err)
		if !ok {
			return "", "", err
		}
		return "", r, nil
	}
	return tok, "", nil
}

func (s *Service) failed(ctx context.Context, out Outcome, at time.Time, r syncstatus.Reason) (Outcome, error) {
	out.Reason = r
	if err := s.State.Failed(ctx, out.Store, at, r); err != nil {
		return out, err
	}
	s.publish(out.Store)
	return out, nil
}

// mirror は、remote への同期に成功したあと、ミラーへ順に送り、送れなかった数を返す(06 のミラーの段落)。
// ミラーに送れなくても、Store の状態は変えない。
func (s *Service) mirror(ctx context.Context, t port.SyncTarget, tok string) (int, error) {
	failures := 0
	for _, m := range t.Mirrors {
		var r syncstatus.Reason
		if err := s.Replicator.Mirror(ctx, t.ID, port.Remote{URL: m, Token: tok}); err != nil {
			r = remoteReason(t.ID, err)
			failures++
			if r == syncstatus.AuthFailed {
				s.Tokens.StoreRejected(ctx, t.ID)
			}
		}
		if err := s.State.Mirrored(ctx, t.ID, m, s.Clock.Now(), r); err != nil {
			return failures, err
		}
	}
	return failures, nil
}
