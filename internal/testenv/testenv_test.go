package testenv

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestMain is also the child of TestSweepRemovesTheDirectoryOfAGoneProcess:
// with TESTENV_CHILD set it makes its directory, says where, and waits to be
// killed.
func TestMain(m *testing.M) {
	if os.Getenv("TESTENV_CHILD") == "1" {
		if _, err := Setup(); err != nil {
			os.Exit(2)
		}
		dir := os.Getenv("TMPDIR")
		// A read-only directory with a file in it, as some tests leave: the
		// file cannot be removed until the directory is writable again.
		ro := filepath.Join(dir, "ro")
		if os.Mkdir(ro, 0o700) != nil || os.WriteFile(filepath.Join(ro, "f"), nil, 0o600) != nil || os.Chmod(ro, 0o500) != nil {
			os.Exit(2)
		}
		fmt.Println(dir)
		time.Sleep(time.Hour)
	}
	Main(m)
}

func TestSetupPointsTMPDIRAtTheProcess(t *testing.T) {
	t.Parallel()
	want := prefix + strconv.Itoa(os.Getpid())
	if got := filepath.Base(os.TempDir()); got != want {
		t.Fatalf("TMPDIR is %s, want a directory named %s", os.TempDir(), want)
	}
	if !strings.HasPrefix(t.TempDir(), os.TempDir()) {
		t.Errorf("t.TempDir is not in %s", os.TempDir())
	}
}

func TestSweepRemovesTheDirectoryOfAGoneProcess(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^$") //nolint:gosec // this test binary
	cmd.Env = append(os.Environ(), "TESTENV_CHILD=1", "TMPDIR="+base)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// The child sleeps for an hour: kill it even if the test fails before the
	// kill below. The kill must also happen before Sweep, so the function can
	// run twice; the Once makes the second call a no-op.
	var once sync.Once
	killChild := func() {
		once.Do(func() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		})
	}
	t.Cleanup(killChild)
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	child := strings.TrimSpace(line)
	if filepath.Dir(child) != base {
		t.Fatalf("the child made %s, not in %s", child, base)
	}
	killChild()

	alive := filepath.Join(base, prefix+strconv.Itoa(os.Getpid()))
	other := filepath.Join(base, prefix+"x")
	for _, d := range []string{alive, other} {
		if err := os.Mkdir(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	Sweep(base)
	if _, err := os.Stat(child); !os.IsNotExist(err) {
		t.Errorf("the directory of the killed child is still there: %v", err)
	}
	for _, d := range []string{alive, other} {
		if _, err := os.Stat(d); err != nil {
			t.Errorf("%s was removed: %v", d, err)
		}
	}
}

func TestWaitFor(t *testing.T) {
	t.Parallel()
	n := 0
	WaitFor(t, "three looks", func() bool { n++; return n == 3 })
	if n != 3 {
		t.Errorf("cond was called %d times", n)
	}
}
