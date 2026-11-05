package store

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/lib/osfs"
)

// acquire takes the lock file at path as another process does: it makes the
// directories above it and waits up to wait.
func acquire(path string, wait time.Duration) (*osfs.Lock, error) {
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	return lockFile(path, wait)
}

func TestLockExcludes(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), ".hedgehug", "local", "lock")
	l, err := acquire(path, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	// A second handle in the same process must not get the lock either.
	if _, err := acquire(path, 20*time.Millisecond); !errors.Is(err, ErrLocked) {
		t.Fatalf("second acquire: err = %v, want ErrLocked", err)
	}
	if err := l.Release(); err != nil {
		t.Fatal(err)
	}
	l2, err := acquire(path, time.Second)
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	if err := l2.Release(); err != nil {
		t.Fatal(err)
	}
}

// TestLockHelperProcess is not a test: TestLockReleasedOnKill runs the test
// binary again with this test selected, to hold the lock in another process.
func TestLockHelperProcess(t *testing.T) {
	t.Parallel()
	path := os.Getenv("HEDGEHUG_LOCK_HELPER")
	if path == "" {
		t.Skip("run by TestLockReleasedOnKill")
	}
	if _, err := acquire(path, time.Second); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("locked")
	time.Sleep(time.Hour) // not select{}: the runtime would call that a deadlock
}

func TestLockReleasedOnKill(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "lock")
	cmd := exec.Command(os.Args[0], "-test.run=^TestLockHelperProcess$")
	cmd.Env = append(os.Environ(), "HEDGEHUG_LOCK_HELPER="+path)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil || line != "locked\n" {
		t.Fatalf("helper said %q, %v", line, err)
	}

	if _, err := acquire(path, 20*time.Millisecond); !errors.Is(err, ErrLocked) {
		t.Fatalf("acquire while the helper holds the lock: err = %v, want ErrLocked", err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	l, err := acquire(path, 5*time.Second)
	if err != nil {
		t.Fatalf("acquire after the helper was killed: %v", err)
	}
	if err := l.Release(); err != nil {
		t.Fatal(err)
	}
}
