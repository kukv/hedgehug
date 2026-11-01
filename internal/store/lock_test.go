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
)

func TestLockExcludes(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".hedgehug", "local", "lock")
	l, err := acquire(path, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	// A second handle in the same process must not get the lock either.
	if _, err := acquire(path, 100*time.Millisecond); !errors.Is(err, ErrLocked) {
		t.Fatalf("second acquire: err = %v, want ErrLocked", err)
	}
	if err := l.release(); err != nil {
		t.Fatal(err)
	}
	l2, err := acquire(path, time.Second)
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	if err := l2.release(); err != nil {
		t.Fatal(err)
	}
}

// TestLockHelperProcess is not a test: TestLockReleasedOnKill runs the test
// binary again with this test selected, to hold the lock in another process.
func TestLockHelperProcess(t *testing.T) {
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

	if _, err := acquire(path, 100*time.Millisecond); !errors.Is(err, ErrLocked) {
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
	if err := l.release(); err != nil {
		t.Fatal(err)
	}
}

func TestWithFileLock(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "auth.lock")
	inside := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_ = WithFileLock(p, time.Second, func() error {
			close(inside)
			<-release
			return nil
		})
	}()
	<-inside
	if err := WithFileLock(p, 100*time.Millisecond, func() error { return nil }); !errors.Is(err, ErrLocked) {
		t.Errorf("second lock = %v, want ErrLocked", err)
	}
	close(release)
	want := errors.New("from fn")
	if err := WithFileLock(p, time.Second, func() error { return want }); !errors.Is(err, want) {
		t.Errorf("err = %v, want fn's", err)
	}
}
