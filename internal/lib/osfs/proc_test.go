package osfs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// detachedChild is set in the environment of the child that
// TestStartDetached starts: the test binary itself, so that the test needs no
// shell on PATH (CI runs the tests with a PATH that has neither git nor sh).
const detachedChild = "OSFS_TEST_DETACHED_CHILD"

// Serial: it sets an environment variable, which the child inherits.
func TestStartDetached(t *testing.T) {
	if os.Getenv(detachedChild) == "1" {
		fmt.Fprintln(os.Stderr, "detached")
		return
	}
	t.Setenv(detachedChild, "1")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "log")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := StartDetached(exe, []string{"-test.run=^TestStartDetached$"}, f); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		b, err := os.ReadFile(p)
		if err == nil && strings.Contains(string(b), "detached") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the log has %q, want the child's stderr", b)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
