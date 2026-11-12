package sysclock_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/infra/sysclock"
)

func TestIDsAreSortedULIDs(t *testing.T) {
	var ids sysclock.IDs
	prev := ids.New()
	for range 1000 {
		id := ids.New()
		if len(id) != 26 || id <= prev {
			t.Fatalf("id %q after %q", id, prev)
		}
		prev = id
	}
}

// 記録の at はミリ秒までなので、時計もミリ秒で切り捨てる。読み戻した時刻と == で比べられるよう、
// 単調時計の読みも持たない。
func TestClockIsMillisecondsWithoutMonotonic(t *testing.T) {
	for range 100 {
		now := (sysclock.Clock{}).Now()
		if now.Nanosecond()%int(time.Millisecond) != 0 || now != now.Round(0) {
			t.Fatalf("Now() = %s", now)
		}
	}
}

func TestClockHasLocalZone(t *testing.T) {
	if got := (sysclock.Clock{}).Now().Location(); got != time.Local {
		t.Errorf("location = %s", got)
	}
}
