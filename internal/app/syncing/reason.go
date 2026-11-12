package syncing

import (
	"errors"
	"log/slog"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// tokenReason は、トークンを得られなかった理由。*ghcall.Failure でなければ false(このマシンの失敗)。
// 読めない・拒否・認証エラーは「認証に失敗しました」(決定 C2-9)、GitHub に届かないときは「接続できません」。
func tokenReason(err error) (syncstatus.Reason, bool) {
	var f *ghcall.Failure
	if !errors.As(err, &f) {
		return "", false
	}
	switch f.Kind {
	case ghcall.NoAccount:
		return syncstatus.NoAccount, true
	case ghcall.Unreachable, ghcall.RateLimited:
		return syncstatus.Unreachable, true
	}
	return syncstatus.AuthFailed, true
}

var remoteReasons = []struct {
	err    error
	reason syncstatus.Reason
}{
	{port.ErrAuthFailed, syncstatus.AuthFailed},
	{port.ErrRemoteUnreadable, syncstatus.Unreadable},
	{port.ErrWriteDenied, syncstatus.WriteDenied},
	{port.ErrRemoteNotStore, syncstatus.NotStore},
	{port.ErrRemoteNewerFormat, syncstatus.NewerFormat},
	{port.ErrNoCommonHistory, syncstatus.Unrelated},
	{port.ErrRemoteRewritten, syncstatus.Rewritten},
	{port.ErrSendConflict, syncstatus.SendConflict},
	{port.ErrMirrorUnrelated, syncstatus.MirrorUnrelated},
	{port.ErrMirrorAhead, syncstatus.MirrorAhead},
}

// remoteReason は、Replicator の失敗の理由。どれにも当たらなければ「remote に接続できません」
// (06: 途中で切れたときは、接続できないと同じ扱い)。
// 理由が決まらずに接続できないとした失敗は、原因を調べられるようログに残す。
func remoteReason(id port.StoreID, err error) syncstatus.Reason {
	for _, r := range remoteReasons {
		if errors.Is(err, r.err) {
			return r.reason
		}
	}
	if !errors.Is(err, port.ErrUnreachable) {
		slog.Warn("同期の失敗を接続できないとして扱います", "store", id, "error", err)
	}
	return syncstatus.Unreachable
}
