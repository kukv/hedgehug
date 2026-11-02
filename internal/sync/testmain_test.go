package sync

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/testenv"
)

func TestMain(m *testing.M) {
	// A rejected push is tried again at once (it waits 500ms, doubling).
	retryWait = time.Millisecond
	testenv.Main(m)
}
