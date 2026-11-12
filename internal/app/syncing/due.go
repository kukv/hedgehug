package syncing

import (
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
)

// eligible は、自動で同期する対象か(06「自動で同期する」: remote があり、非表示でなく、アカウントがある)。
func eligible(t port.SyncTarget) bool { return t.Remote != "" && !t.Hidden && t.Account != "" }

// due は、間隔か書き込み後の契機で、いま同期するか(06「自動で同期する」)。rec は記録、wrote は最後の
// 保存の時刻(なければゼロ)。同期エラーの Store は、書き込み後でも、最後に試みてから間隔が過ぎるまで待つ。
func due(now time.Time, rec port.SyncRecord, wrote time.Time, t port.SyncTiming) bool {
	if rec.LastAttempt.IsZero() || now.Sub(rec.LastAttempt) >= t.Interval {
		return true
	}
	if wrote.IsZero() || now.Sub(wrote) < t.AfterWrite {
		return false
	}
	return rec.Reason == ""
}
