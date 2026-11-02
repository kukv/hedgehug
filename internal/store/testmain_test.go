package store

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/testenv"
)

func TestMain(m *testing.M) {
	// Waits for a lock look again every 5ms, so that a test of a lock that
	// cannot be taken ends soon after its wait.
	lockPoll = 5 * time.Millisecond
	testenv.Main(m)
}
