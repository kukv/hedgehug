package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/testenv"
)

// waitChildDone waits until the detached child of te has cleared
// pending_since and let go of its locks, so that nothing of it runs while the
// test directories are removed.
func waitChildDone(t *testing.T, te *testEnv) {
	t.Helper()
	state := filepath.Join(te.storeDir("org"), ".hedgehug", "local", "sync.json")
	s, err := store.Open(te.storeDir("org"))
	if err != nil {
		t.Fatal(err)
	}
	testenv.WaitFor(t, "the background child to finish", func() bool {
		b, err := os.ReadFile(state) //nolint:gosec // the state of the test
		if err != nil || strings.Contains(string(b), "pending_since") {
			return false
		}
		free := true
		for _, l := range []string{"bg.lock", "sync.lock"} {
			if s.WithLocalLock(l, 0, func() error { return nil }) != nil {
				free = false
			}
		}
		return free
	})
}

// settled is twoUsers with alice's sync done after her last write: the clock
// of the tests stands still, so a write at the instant a sync begins is
// still pending after it.
func settled(t *testing.T) *testEnv {
	t.Helper()
	alice, _, _ := twoUsers(t)
	alice.now = alice.now.Add(time.Second)
	alice.ok("sync")
	alice.now = alice.now.Add(time.Second)
	return alice
}

func TestWriteStartsBackgroundSync(t *testing.T) {
	alice := settled(t)
	var started []string
	defer stubSpawn(func(args []string) { started = append(started, strings.Join(args, " ")) })()
	alice.env["HEDGEHUG_AUTO_SYNC"] = "1"
	alice.ok("add", "x")
	if len(started) != 1 || !strings.Contains(started[0], "sync --background --store org") {
		t.Errorf("started = %v", started)
	}
	state, err := os.ReadFile(filepath.Join(alice.storeDir("org"), ".hedgehug", "local", "sync.json"))
	if err != nil || !strings.Contains(string(state), "pending_since") {
		t.Errorf("sync.json = %s, err = %v", state, err)
	}
}

func TestWriteWithoutRemoteStartsNothing(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "solo", "--prefix", "SOL")
	started := 0
	defer stubSpawn(func([]string) { started++ })()
	te.env["HEDGEHUG_AUTO_SYNC"] = "1"
	te.ok("add", "x")
	if started != 0 {
		t.Error("started a sync for a store with no remote")
	}
}

func TestStoreFlagOnAStoreWithoutRemoteStartsNothing(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "solo", "--prefix", "SOL")
	started := 0
	defer stubSpawn(func([]string) { started++ })()
	te.env["HEDGEHUG_AUTO_SYNC"] = "1"
	te.ok("--store", "solo", "ls")
	if started != 0 {
		t.Errorf("started %d for a store with no remote", started)
	}
}

func TestSyncBackgroundRecordsAFailureBeforeTheRun(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	remote := te.bare("shared")
	te.ok("store", "init", "org", "--prefix", "ORG", "--remote", remote)
	// No account and no author in the user's gitconfig: the options of the
	// sync cannot be made.
	if err := os.Remove(filepath.Join(te.home, ".gitconfig")); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := te.run("sync", "--background", "--store", "org"); code != 0 || !strings.Contains(errs, "sync org:") {
		t.Fatalf("code = %d, stderr = %s", code, errs)
	}
	_, _, errs := te.run("ls")
	if !strings.Contains(errs, "hedgehug: background sync of org failed:") {
		t.Errorf("stderr = %s", errs)
	}
}

func TestReadStartsBackgroundSyncOnlyWhenDue(t *testing.T) {
	alice := settled(t)
	var started int
	defer stubSpawn(func([]string) { started++ })()
	alice.env["HEDGEHUG_AUTO_SYNC"] = "1"
	alice.ok("ls")
	if started != 0 {
		t.Errorf("started %d for a store synced a moment ago", started)
	}
	alice.now = alice.now.Add(3 * time.Minute)
	alice.ok("ls")
	if started != 1 {
		t.Errorf("started %d after the interval", started)
	}
}

func TestSyncStartsNothing(t *testing.T) {
	alice, _, _ := twoUsers(t)
	var started int
	defer stubSpawn(func([]string) { started++ })()
	alice.env["HEDGEHUG_AUTO_SYNC"] = "1"
	alice.now = alice.now.Add(time.Hour)
	alice.ok("sync", "--status")
	alice.ok("sync")
	if started != 0 {
		t.Errorf("started %d", started)
	}
}

func TestStartReportsBackgroundError(t *testing.T) {
	t.Parallel()
	alice, _, _ := twoUsers(t)
	alice.setSyncError("org", "remote unreachable")
	_, _, errs := alice.run("ls")
	if !strings.Contains(errs, "hedgehug: background sync of org failed: remote unreachable (run hedgehug sync)") {
		t.Errorf("stderr = %s", errs)
	}
	if _, _, errs := alice.run("ls"); strings.Contains(errs, "background sync") {
		t.Error("reported twice")
	}
}

func TestForegroundSyncFailureIsNotReportedAgain(t *testing.T) {
	t.Parallel()
	alice, _, remote := twoUsers(t)
	if err := os.RemoveAll(remote); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := alice.run("sync"); code != 1 {
		t.Fatalf("sync with no remote: code %d", code)
	}
	if _, _, errs := alice.run("ls"); strings.Contains(errs, "background sync") {
		t.Errorf("a failure the sync told of is told of again: %s", errs)
	}
}

func TestSyncBackgroundNeedsAStore(t *testing.T) {
	t.Parallel()
	alice, _, _ := twoUsers(t)
	if code, _, _ := alice.run("sync", "--background"); code != 2 {
		t.Errorf("code = %d", code)
	}
}

func TestSyncBackgroundPushesAfterPushInterval(t *testing.T) {
	t.Parallel()
	alice, bob, _ := twoUsers(t)
	alice.writeSync("50ms", "50ms")
	alice.ok("add", "bg")
	alice.now = time.Now() // the wait is counted on the clock of the command
	if code, _, errs := alice.run("sync", "--background", "--store", "org"); code != 0 {
		t.Fatalf("code = %d: %s", code, errs)
	}
	bob.ok("sync")
	if !strings.Contains(bob.ok("ls"), "bg") {
		t.Error("the background sync did not push")
	}
	state, _ := os.ReadFile(filepath.Join(alice.storeDir("org"), ".hedgehug", "local", "sync.json"))
	if strings.Contains(string(state), "pending_since") {
		t.Errorf("sync.json = %s", state)
	}
}

func TestBackgroundSyncReachesRemote(t *testing.T) {
	t.Parallel()
	// The real binary: a write starts a detached child that pushes after push_interval.
	bin := buildHedgehug(t)
	alice, bob, _ := twoUsers(t)
	// twoUsers synced on the test clock, which leaves pending_since in the
	// past. A sync on the real clock clears it, and an interval that never
	// falls due keeps the command from starting a child before it writes: that
	// child would sync while the write is half done, or clear the write's
	// pending_since without having pushed it.
	alice.writeSync("1h", "50ms")
	alice.now = time.Now()
	alice.ok("sync")
	delete(alice.env, "HEDGEHUG_AUTO_SYNC") // unset, as a user has it
	alice.runBinary(bin, "add", "bg")
	// Bob syncs once the child is done: a fetch that reads the ref while the
	// push writes it sees an empty file.
	waitChildDone(t, alice)
	bob.ok("sync")
	if !strings.Contains(bob.ok("ls"), "bg") {
		t.Error("the background sync did not push")
	}
}
