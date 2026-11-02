package server

import (
	"sync"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/testenv"
)

// fakeClock is a Clock that moves only when Advance is called.
type fakeClock struct {
	mu      sync.Mutex
	now     time.Time
	waiters []fakeWaiter
	afters  int // calls of After
}

type fakeWaiter struct {
	at time.Time
	c  chan time.Time
}

func newFakeClock(t time.Time) *fakeClock { return &fakeClock{now: t} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) After(d time.Duration) <-chan time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.afters++
	ch := make(chan time.Time, 1)
	if d <= 0 {
		ch <- c.now
		return ch
	}
	c.waiters = append(c.waiters, fakeWaiter{c.now.Add(d), ch})
	return ch
}

// afterCalls is how many times After was called: the loop calls it each time
// it parks, so a new call means the loop looked again.
func (c *fakeClock) afterCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.afters
}

// Advance moves the clock by d and fires the waiters that are due.
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	kept := c.waiters[:0]
	for _, w := range c.waiters {
		if !w.at.After(c.now) {
			w.c <- c.now
			continue
		}
		kept = append(kept, w)
	}
	c.waiters = kept
}

// waitWaiter waits until something waits for the time at, so that a test
// advances the clock only once the loop is parked on it.
func (c *fakeClock) waitWaiter(t *testing.T, at time.Time) {
	t.Helper()
	eventually(t, "a wait until "+at.Format(time.RFC3339), func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		for _, w := range c.waiters {
			if w.at.Equal(at) {
				return true
			}
		}
		return false
	})
}

// eventually waits up to 10 seconds of real time for cond.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	testenv.WaitFor(t, what, cond)
}

func TestFakeClock(t *testing.T) {
	t.Parallel()
	c := newFakeClock(time.Unix(0, 0))
	ch := c.After(time.Minute)
	c.Advance(30 * time.Second)
	select {
	case <-ch:
		t.Fatal("fired early")
	default:
	}
	c.Advance(30 * time.Second)
	select {
	case <-ch:
	default:
		t.Fatal("did not fire")
	}
}
