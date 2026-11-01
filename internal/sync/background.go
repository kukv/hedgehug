package sync

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	gosync "sync"
	"time"

	"github.com/kukv/hedgehug/internal/store"
)

const (
	// bgLock is held by the background sync of a store, from its start to its
	// end, so that only one waits at a time.
	bgLock = "bg.lock"
	// logName is where the stderr of the background sync goes, in
	// .hedgehug/local.
	logName = "sync.log"
	// logMax is the size of sync.log above which it keeps only its last
	// half, when a child starts and after each write the child makes.
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
	return spawn(exe, []string{"sync", "--background", "--store", name}, logPath(s))
}

func logPath(s *store.Store) string {
	return filepath.Join(s.Dir(), ".hedgehug", "local", logName)
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
	return BackgroundThen(s, opt, pushInterval, func() (func() error, error) { return nil, run() })
}

// BackgroundThen is Background whose run may return a function to call once
// bg.lock is let go, the last thing the background sync does (run says it
// is done with the loop by returning one). It is for the swap of a rebuilt
// store, which cannot happen while a file of the store is held.
func BackgroundThen(s *store.Store, opt Options, pushInterval time.Duration, run func() (func() error, error)) error {
	now := opt.Now
	if now == nil {
		now = time.Now
	}
	var then func() error
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
			if then, err = run(); err != nil || then != nil {
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
	if err != nil || then == nil {
		return err
	}
	return then()
}

// TakeError returns the last background error once (it sets error_reported).
// It reads first, so a store with nothing to report is not written to.
func TakeError(s *store.Store) (string, error) {
	st, err := ReadState(s)
	if err != nil || st.LastError == "" || st.ErrorReported {
		return "", err
	}
	var msg string
	_, err = UpdateState(s, func(st *State) {
		if st.LastError != "" && !st.ErrorReported { // another command may have taken it
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

// BoundLog returns w, which is the stderr of the child that Start started,
// as a writer that keeps sync.log within logMax while the child runs.
func BoundLog(s *store.Store, w io.Writer) *BoundedLog {
	l := &BoundedLog{w: w, path: logPath(s)}
	if f, ok := w.(*os.File); ok {
		a, aerr := f.Stat()
		b, berr := os.Stat(l.path)
		if aerr == nil && berr == nil && os.SameFile(a, b) {
			l.own = f
		}
	}
	return l
}

// BoundedLog trims the log after each write that makes it larger than
// logMax. w appends to the file at path. It can let go of that file for the
// swap of a rebuilt store, which replaces the directory the file is in, and
// open it again in the new one.
type BoundedLog struct {
	mu   gosync.Mutex
	w    io.Writer
	path string
	// own is the file behind w when it is the sync.log of the store; nil
	// otherwise, then Release and Reopen do nothing.
	own      *os.File
	nul      *os.File // where the standard error went while released
	released bool
	held     []byte // written while released
}

func (b *BoundedLog) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.released {
		b.held = append(b.held, p...)
		return len(p), nil
	}
	n, err := b.w.Write(p)
	if terr := trimLog(b.path); err == nil {
		err = terr
	}
	return n, err
}

// Release lets go of sync.log: the file is closed, and when it is the
// standard error of the process that points at os.DevNull. What is written
// meanwhile is kept for Reopen.
func (b *BoundedLog) Release() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.own == nil || b.released {
		return nil
	}
	if b.own != os.Stderr {
		// Released only when the close worked: a log that is still open goes
		// on taking lines.
		if err := b.own.Close(); err != nil {
			return err
		}
		b.released = true
		return nil
	}
	nul, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	if err := redirectStderr(nul); err != nil {
		_ = nul.Close()
		return err
	}
	b.nul, b.released = nul, true // nul stays open until Reopen: it may be the standard handle
	return nil
}

// Close lets go of the null device a Release left the standard error on,
// at the end of the run when no Reopen came. The log is not used after.
func (b *BoundedLog) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.nul == nil {
		return nil
	}
	err := b.nul.Close()
	b.nul = nil
	return err
}

// Reopen opens sync.log again, at the same path, which may be in another
// directory now, and writes what was kept. The standard error, if it was
// the file, points at it again.
func (b *BoundedLog) Reopen() error { return b.ReopenAt("") }

// ReopenAt is Reopen for the sync.log of the store in the directory dir, when
// the store is not at its path (a swap that stopped); "" is the path it has.
func (b *BoundedLog) ReopenAt(dir string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.released {
		return nil
	}
	if dir != "" {
		b.path = filepath.Join(dir, ".hedgehug", "local", logName)
	}
	f, err := openLog(b.path)
	if err != nil {
		return err
	}
	if b.own == os.Stderr {
		if err := redirectStderr(f); err != nil {
			_ = f.Close()
			return err
		}
		// The standard error is the log again; nul is no longer used.
		if b.nul != nil {
			_ = b.nul.Close()
			b.nul = nil
		}
	}
	b.w, b.own, b.released = f, f, false
	held := b.held
	b.held = nil
	if len(held) > 0 {
		_, err = f.Write(held)
	}
	return err
}

// trimLog keeps the last half of a log over logMax. It rewrites the file in
// place, so that a child that has it open for appending goes on writing to it.
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
	return os.WriteFile(path, b[len(b)/2:], 0o600) //nolint:gosec // G703: the path is built from the store root
}
