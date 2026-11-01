package gitrepo

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/testenv"
)

func TestMain(m *testing.M) {
	indexLockPoll = 5 * time.Millisecond
	testenv.Main(m)
}
