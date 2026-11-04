package scenario

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/lib/testenv"
)

// buildHedgehug builds the command once for all tests and returns the path of
// the binary, which is named hedgehug so that a merge driver finds it on PATH.
func buildHedgehug(t *testing.T) string {
	t.Helper()
	return testenv.Build(t, "github.com/kukv/hedgehug/cmd/hedgehug", "hedgehug")
}

func TestMain(m *testing.M) {
	// Everything the tests make, the binaries included, goes in a directory
	// of this process that cleanup removes.
	cleanup, err := testenv.Setup()
	if err != nil {
		fmt.Fprintln(os.Stderr, "testenv:", err)
		os.Exit(1)
	}
	// A program built with -race sleeps a second when it exits, by default,
	// so that goroutines still running at the exit can be caught in a race.
	// The binaries the tests run leave no goroutine of their own work running
	// when they exit, so for them the sleep only makes each run a second
	// longer.
	_ = os.Setenv("GORACE", strings.TrimSpace(os.Getenv("GORACE")+" atexit_sleep_ms=0"))
	// hedgehug is on PATH, as doctor expects of a store with a merge driver.
	pathDir, err := os.MkdirTemp("", "hedgehug-path-")
	if err == nil {
		if err := os.WriteFile(filepath.Join(pathDir, "hedgehug"), nil, 0o700); err == nil { //nolint:gosec // an empty file that only has to be found
			_ = os.Setenv("PATH", pathDir+string(os.PathListSeparator)+os.Getenv("PATH"))
		}
	}
	code := m.Run()
	cleanup()
	os.Exit(code)
}
