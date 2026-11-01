package sync

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/store"
)

// holdLocal takes the local lock name, as a waiting child would, until
// release is called.
func holdLocal(t *testing.T, s *store.Store, name string) (release func()) {
	t.Helper()
	held, done := make(chan struct{}), make(chan struct{})
	go func() {
		_ = s.WithLocalLock(name, time.Second, func() error {
			close(held)
			<-done
			return nil
		})
	}()
	<-held
	return func() { close(done) }
}

func TestStartRespectsAutoSyncAndLock(t *testing.T) {
	c := newCluster(t, 1)
	calls := 0
	defer SetSpawn(func(string, []string, string) error { calls++; return nil })()
	_ = Start(c.stores[0], "s", func(k string) string { return map[string]string{"HEDGEHUG_AUTO_SYNC": "0"}[k] })
	if calls != 0 {
		t.Error("started with HEDGEHUG_AUTO_SYNC=0")
	}
	if err := Start(c.stores[0], "s", func(string) string { return "" }); err != nil || calls != 1 {
		t.Errorf("calls = %d, err = %v", calls, err)
	}
	release := holdLocal(t, c.stores[0], "bg.lock") // as a waiting child would
	defer release()
	if err := Start(c.stores[0], "s", func(string) string { return "" }); err != nil || calls != 1 {
		t.Errorf("started while another background sync waits: calls = %d, err = %v", calls, err)
	}
}

func TestStartGivesTheChildItsCommand(t *testing.T) {
	c := newCluster(t, 1)
	var args []string
	var log string
	defer SetSpawn(func(_ string, a []string, l string) error { args, log = a, l; return nil })()
	if err := Start(c.stores[0], "work", func(string) string { return "" }); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(args, " "); got != "sync --background --store work" {
		t.Errorf("args = %q", got)
	}
	if want := filepath.Join(c.stores[0].Dir(), ".hedgehug", "local", "sync.log"); log != want {
		t.Errorf("log = %q, want %q", log, want)
	}
}

func TestBackgroundWaitsAndRuns(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	now := time.Now()
	_ = MarkPending(c.stores[0], now)
	runs := 0
	err := Background(c.stores[0], c.opts[0], 50*time.Millisecond, func() error {
		runs++
		if time.Since(now) < 50*time.Millisecond {
			t.Error("ran before push_interval")
		}
		_, err := UpdateState(c.stores[0], func(st *State) { st.PendingSince = time.Time{} })
		return err
	})
	if err != nil || runs != 1 {
		t.Errorf("runs = %d, err = %v", runs, err)
	}
}

func TestBackgroundRunsAtOnceWithNothingPending(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	start := time.Now()
	runs := 0
	if err := Background(c.stores[0], c.opts[0], time.Hour, func() error { runs++; return nil }); err != nil || runs != 1 {
		t.Errorf("runs = %d, err = %v", runs, err)
	}
	if time.Since(start) > time.Minute {
		t.Error("waited with nothing pending")
	}
}

func TestBackgroundRepeatsWhileSomethingIsPending(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	_ = MarkPending(c.stores[0], time.Now())
	runs := 0
	err := Background(c.stores[0], c.opts[0], time.Millisecond, func() error {
		runs++
		if runs == 1 { // a write during the first run
			return nil
		}
		_, err := UpdateState(c.stores[0], func(st *State) { st.PendingSince = time.Time{} })
		return err
	})
	if err != nil || runs != 2 {
		t.Errorf("runs = %d, err = %v", runs, err)
	}
}

func TestBackgroundEndsOnAFailedRun(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	_ = MarkPending(c.stores[0], time.Now())
	boom := errors.New("boom")
	runs := 0
	if err := Background(c.stores[0], c.opts[0], time.Millisecond, func() error { runs++; return boom }); !errors.Is(err, boom) || runs != 1 {
		t.Errorf("runs = %d, err = %v", runs, err)
	}
}

func TestBackgroundLeavesToTheOneThatWaits(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	release := holdLocal(t, c.stores[0], "bg.lock")
	defer release()
	runs := 0
	if err := Background(c.stores[0], c.opts[0], time.Millisecond, func() error { runs++; return nil }); err != nil || runs != 0 {
		t.Errorf("runs = %d, err = %v", runs, err)
	}
}

func TestMarkPendingKeepsTheFirst(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	first := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	_ = MarkPending(c.stores[0], first)
	_ = MarkPending(c.stores[0], first.Add(time.Hour))
	st, _ := ReadState(c.stores[0])
	if !st.PendingSince.Equal(first) {
		t.Errorf("pending_since = %v", st.PendingSince)
	}
}

func TestDue(t *testing.T) {
	t.Parallel()
	now := time.Now()
	if !Due(State{PendingSince: now}, now, time.Minute) {
		t.Error("pending: not due")
	}
	if Due(State{LastSuccess: now.Add(-time.Second)}, now, time.Minute) {
		t.Error("fresh: due")
	}
	if !Due(State{LastSuccess: now.Add(-2 * time.Minute)}, now, time.Minute) {
		t.Error("stale: not due")
	}
	if !Due(State{}, now, time.Minute) {
		t.Error("never synced: not due")
	}
	failed := State{PendingSince: now, LastError: "x", LastAttempt: now.Add(-time.Second)}
	if Due(failed, now, time.Minute) {
		t.Error("failed a moment ago: due")
	}
	failed.LastAttempt = now.Add(-2 * time.Minute)
	if !Due(failed, now, time.Minute) {
		t.Error("failed long ago: not due")
	}
}

func TestTakeErrorOnce(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	_, _ = UpdateState(c.stores[0], func(st *State) { st.LastError = "boom" })
	if e, _ := TakeError(c.stores[0]); e != "boom" {
		t.Errorf("first = %q", e)
	}
	if e, _ := TakeError(c.stores[0]); e != "" {
		t.Errorf("second = %q", e)
	}
}

func TestTakeErrorWritesNothingWhenThereIsNone(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	if err := c.stores[0].RemoveLocal("sync.json"); err != nil {
		t.Fatal(err)
	}
	if e, err := TakeError(c.stores[0]); e != "" || err != nil {
		t.Fatalf("TakeError = %q, %v", e, err)
	}
	if _, err := c.stores[0].ReadLocal("sync.json"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("sync.json was written: %v", err)
	}
}

func TestOpenLogKeepsTheLastHalfOfABigLog(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "local", "sync.log")
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	big := strings.Repeat("a", logMax) + strings.Repeat("b", 100)
	if err := os.WriteFile(p, []byte(big), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := openLog(p)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("new\n")
	_ = f.Close()
	b, _ := os.ReadFile(p)
	if len(b) > logMax/2+200 || !strings.HasSuffix(string(b), strings.Repeat("b", 100)+"new\n") {
		t.Errorf("log is %d bytes, ends %q", len(b), b[len(b)-20:])
	}
	small := filepath.Join(filepath.Dir(p), "small.log")
	_ = os.WriteFile(small, []byte("old\n"), 0o600)
	f, err = openLog(small)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("new\n")
	_ = f.Close()
	if b, _ := os.ReadFile(small); string(b) != "old\nnew\n" {
		t.Errorf("small log = %q", b)
	}
}

func TestBoundLogKeepsTheLogWithinLogMax(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "local", "sync.log")
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // as the child has it from openLog
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	w := &boundedLog{w: f, path: p}
	line := strings.Repeat("x", 99) + "\n"
	for range 3 * logMax / len(line) {
		if _, err := w.Write([]byte(line)); err != nil {
			t.Fatal(err)
		}
		if fi, err := os.Stat(p); err != nil || fi.Size() > logMax {
			t.Fatalf("log is over logMax: %v, %v", fi, err)
		}
	}
	if _, err := w.Write([]byte("last\n")); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if len(b) > logMax || !strings.HasSuffix(string(b), line+"last\n") {
		t.Errorf("log is %d bytes, ends %q", len(b), b[len(b)-20:])
	}
}
