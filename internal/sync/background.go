package sync

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/kukv/hedgehug/internal/atomicfile"
	"github.com/kukv/hedgehug/internal/store"
)

const (
	// bgLock is held by the background sync of a store, from its start to its
	// end, so that only one waits at a time.
	bgLock = "bg.lock"
	// logName is where the stderr of the background sync goes, in
	// .hedgehug/local.
	logName = "sync.log"
	// logMax is the size of sync.log above which a new child keeps only its
	// last half.
	logMax = 1 << 20
)

// MarkPending sets pending_since if it is empty. Writing commands call it
// after a commit.
func MarkPending(s *store.Store, now time.Time) error {
	_, err := UpdateState(s, func(st *State) {
		if st.PendingSince.IsZero() {
			st.PendingSince = now
		}
	})
	return err
}

// Due reports whether a sync should start for a command: something waits to
// be pushed, or the last success is older than interval. A sync that failed
// less than interval ago is not tried again yet, so that a remote that is
// down does not start a process for every command.
func Due(st State, now time.Time, interval time.Duration) bool {
	if st.LastError != "" && now.Sub(st.LastAttempt) < interval {
		return false
	}
	return !st.PendingSince.IsZero() || now.Sub(st.LastSuccess) >= interval
}

// Start starts "hedgehug sync --background --store <name>" detached, unless
// HEDGEHUG_AUTO_SYNC=0 or another background sync holds bg.lock.
func Start(s *store.Store, name string, getenv func(string) string) error {
	if getenv("HEDGEHUG_AUTO_SYNC") == "0" {
		return nil
	}
	// Only a probe: the child takes bg.lock again. If parent and child race
	// for it, one of them gives up and syncs never run twice at once.
	err := s.WithLocalLock(bgLock, 0, func() error { return nil })
	if errors.Is(err, store.ErrLocked) {
		return nil
	}
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return spawn(exe, []string{"sync", "--background", "--store", name}, filepath.Join(s.Dir(), ".hedgehug", "local", logName))
}

// spawn starts the process detached from this one; tests replace it.
var spawn = spawnDetached

// SetSpawn replaces how Start starts the child, and returns the function
// that puts the old one back. It is for tests only, so that they never
// start a process.
func SetSpawn(f func(name string, args []string, logPath string) error) (restore func()) {
	old := spawn
	spawn = f
	return func() { spawn = old }
}

// Background is the body of sync --background: wait for push_interval after
// pending_since, run, repeat while something is pending. run takes sync.lock
// itself. A run that fails
// ends it, since run keeps the error in sync.json for the next command to
// show and to start another.
func Background(s *store.Store, opt Options, pushInterval time.Duration, run func() error) error {
	now := opt.Now
	if now == nil {
		now = time.Now
	}
	err := s.WithLocalLock(bgLock, 0, func() error {
		for {
			st, err := ReadState(s)
			if err != nil {
				return err
			}
			if !st.PendingSince.IsZero() {
				if d := st.PendingSince.Add(pushInterval).Sub(now()); d > 0 {
					time.Sleep(d)
				}
			}
			if err := run(); err != nil {
				return err
			}
			if st, err = ReadState(s); err != nil {
				return err
			}
			if st.PendingSince.IsZero() {
				return nil
			}
		}
	})
	if errors.Is(err, store.ErrLocked) {
		return nil // another one waits already
	}
	return err
}

// TakeError returns the last background error once (it sets error_reported).
func TakeError(s *store.Store) (string, error) {
	var msg string
	_, err := UpdateState(s, func(st *State) {
		if st.LastError != "" && !st.ErrorReported {
			msg = st.LastError
			st.ErrorReported = true
		}
	})
	return msg, err
}

// openLog opens the log of the child for appending. A log over logMax first
// loses its first half.
func openLog(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil { //nolint:gosec // G703: the path is built from the store root
		return nil, err
	}
	if err := trimLog(path); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // G304: the path is built from the store root
}

func trimLog(path string) error {
	fi, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) || (err == nil && fi.Size() <= logMax) {
		return nil
	}
	if err != nil {
		return err
	}
	b, err := os.ReadFile(path) //nolint:gosec // G304: the path is built from the store root
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(path, b[len(b)/2:])
}
