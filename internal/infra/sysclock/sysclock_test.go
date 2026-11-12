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

func TestClockHasLocalZone(t *testing.T) {
	if got := (sysclock.Clock{}).Now().Location(); got != time.Local {
		t.Errorf("location = %s", got)
	}
}
