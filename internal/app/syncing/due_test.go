package syncing

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// 06「自動で同期する」の契機のうち、間隔と書き込み後。
func TestDue(t *testing.T) {
	t0 := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	timing := port.SyncTiming{Auto: true, Interval: 5 * time.Minute, AfterWrite: 30 * time.Second}
	ok := port.SyncRecord{LastAttempt: t0, LastSuccess: t0}
	failed := port.SyncRecord{LastAttempt: t0, Reason: syncstatus.AuthFailed}
	cases := []struct {
		name  string
		now   time.Time
		rec   port.SyncRecord
		wrote time.Time
		want  bool
	}{
		{"never tried", t0, port.SyncRecord{}, time.Time{}, true},
		{"interval not yet", t0.Add(4 * time.Minute), ok, time.Time{}, false},
		{"interval passed", t0.Add(5 * time.Minute), ok, time.Time{}, true},
		{"failed attempts count too", t0.Add(5 * time.Minute), failed, time.Time{}, true},
		{"write, waiting", t0.Add(2*time.Minute + 29*time.Second), ok, t0.Add(2 * time.Minute), false},
		{"write, quiet for 30s", t0.Add(2*time.Minute + 30*time.Second), ok, t0.Add(2 * time.Minute), true},
		// 06 の例: 10:00 に認証エラー、10:02 に保存しても、10:02:30 では同期せず 10:05 まで待つ。
		{"error store waits for the interval", t0.Add(2*time.Minute + 30*time.Second), failed, t0.Add(2 * time.Minute), false},
		{"error store at the interval", t0.Add(5 * time.Minute), failed, t0.Add(2 * time.Minute), true},
	}
	for _, c := range cases {
		if got := due(c.now, c.rec, c.wrote, timing); got != c.want {
			t.Errorf("%s: due = %v", c.name, got)
		}
	}
}
