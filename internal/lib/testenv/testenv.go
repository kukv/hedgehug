// Package testenv is for the tests of the other packages. Main gives the
// tests of a package a temporary directory of their own process and removes
// the directories of test processes that are gone, so that a run stopped half
// way leaves nothing behind once the next one has run. WaitFor waits for a
// condition, and Build builds a command for the tests to run.
package testenv

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// prefix starts the name of the directory of a test process; its pid follows.
const prefix = "hedgehug-test-"

// Main runs the tests of m with Setup and exits with their code.
func Main(m *testing.M) {
	cleanup, err := Setup()
	if err != nil {
		fmt.Fprintln(os.Stderr, "testenv:", err)
		os.Exit(1)
	}
	code := m.Run()
	cleanup()
	os.Exit(code)
}

// Setup removes the directories of test processes that are gone from the
// temporary directory, makes hedgehug-test-<pid> there and points TMPDIR at
// it, so that t.TempDir and os.MkdirTemp put everything in it, children of
// the test included. cleanup removes it and points TMPDIR back.
func Setup() (cleanup func(), err error) {
	base := os.TempDir()
	Sweep(base)
	dir := filepath.Join(base, prefix+strconv.Itoa(os.Getpid()))
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, err
	}
	old, had := os.LookupEnv("TMPDIR")
	if err := os.Setenv("TMPDIR", dir); err != nil {
		return nil, err
	}
	return func() {
		if had {
			_ = os.Setenv("TMPDIR", old)
		} else {
			_ = os.Unsetenv("TMPDIR")
		}
		removeAll(dir)
	}, nil
}

// Sweep removes the hedgehug-test-<pid> directories in base whose process is
// gone. A process it may not signal is taken as alive.
func Sweep(base string) {
	entries, err := os.ReadDir(base)
	if err != nil {
		return
	}
	for _, e := range entries {
		rest, ok := strings.CutPrefix(e.Name(), prefix)
		if !ok || !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(rest)
		if err != nil || pid <= 0 {
			continue
		}
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			removeAll(filepath.Join(base, e.Name()))
		}
	}
}

// removeAll removes dir, making its directories writable first when a test
// left one read-only.
func removeAll(dir string) {
	if os.RemoveAll(dir) == nil {
		return
	}
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		// A directory that could not be read comes with err set; it is the
		// one to make writable.
		if d != nil && d.IsDir() {
			_ = os.Chmod(p, 0o700) //nolint:gosec // G302: a directory of the tests, about to be removed
		}
		return nil
	})
	_ = os.RemoveAll(dir)
}

// WaitFor calls cond every 5ms until it is true, and fails the test when it is
// not within 10 seconds. what names what is waited for.
func WaitFor(t testing.TB, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// builds are the binaries Build made in this process, by package.
var (
	buildsMu sync.Mutex
	builds   = map[string]*built{}
)

type built struct {
	once sync.Once
	path string
	err  error
}

// Build builds the main package pkg (an import path, or a path relative to
// the directory of the test) once in this process, as a binary called name in
// a temporary directory of its own, and returns its path. The build has -race
// when the test binary has it: the packages are then in the build cache from
// the build of the tests, and are not compiled again.
func Build(tb testing.TB, pkg, name string) string {
	tb.Helper()
	buildsMu.Lock()
	b := builds[pkg]
	if b == nil {
		b = &built{}
		builds[pkg] = b
	}
	buildsMu.Unlock()
	b.once.Do(func() {
		dir, err := os.MkdirTemp("", name+"-bin-")
		if err != nil {
			b.err = err
			return
		}
		b.path = filepath.Join(dir, name)
		args := []string{"build"}
		if raceEnabled() {
			args = append(args, "-race")
		}
		args = append(args, "-o", b.path, pkg)
		if out, err := exec.Command("go", args...).CombinedOutput(); err != nil { //nolint:gosec // the arguments are the caller's
			b.err = fmt.Errorf("go build %s: %w\n%s", pkg, err, out)
		}
	})
	if b.err != nil {
		tb.Fatal(b.err)
	}
	return b.path
}

// raceEnabled reports whether this binary was built with -race.
func raceEnabled() bool {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return false
	}
	for _, s := range bi.Settings {
		if s.Key == "-race" && s.Value == "true" {
			return true
		}
	}
	return false
}
