package ops

import (
	"errors"
	"io"
	"testing"

	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// The background sync lets go of bg.lock, like sync.lock and the store lock,
// before the swap: no lock file of the store is held when the directory is
// renamed (Windows cannot rename a directory with an open file).
func TestBackgroundSyncSwapsWithNoLockHeld(t *testing.T) {
	f := newTakeFx(t)
	path := f.path(t)
	f.clone.appendConfig("sync:\n  push_interval: 1ms\n")
	ss := f.clone.ForStore("me")
	var checked bool
	ss.BeforeSwap = func(p string) {
		checked = true
		s, err := store.Open(p)
		if err != nil {
			t.Error(err)
			return
		}
		defer func() { _ = s.Repo().Close() }()
		for _, name := range []string{"bg.lock", "sync.lock", "lock"} {
			if err := s.WithLocalLock(name, 0, func() error { return nil }); err != nil {
				t.Errorf("%s is held at the swap: %v", name, err)
			}
		}
	}
	if err := ss.SyncBackground(io.Discard); err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Fatal("BeforeSwap was not called")
	}
	if left, _ := sync.FindLeftover(path); !left.None() {
		t.Errorf("leftover = %+v", left)
	}
	s, err := ss.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Task(f.gone.ULID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("the purged task = %v", err)
	}
	if got, want := originHead(t, f.remote), mustHead(t, s); got != want {
		t.Errorf("origin is at %s, the clone at %s: the background sync did not push", got, want)
	}
}
