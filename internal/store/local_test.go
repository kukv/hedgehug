package store

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/atomicfile"
)

func TestLocalFiles(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.ReadLocal("index"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing file: err = %v, want ErrNotFound", err)
	}
	if err := s.WriteLocal("index", []byte("data")); err != nil {
		t.Fatal(err)
	}
	b, err := s.ReadLocal("index")
	if err != nil || string(b) != "data" {
		t.Errorf("ReadLocal = %q, %v", b, err)
	}
	// The file is under .hedgehug/local, which git ignores.
	if _, err := atomicfile.ReadFile(s.path(".hedgehug/local/index")); err != nil {
		t.Errorf("file not in .hedgehug/local: %v", err)
	}
	for _, bad := range []string{"", ".", "..", "a/b", `a\b`} {
		if err := s.WriteLocal(bad, nil); err == nil {
			t.Errorf("WriteLocal(%q) succeeded", bad)
		}
		if _, err := s.ReadLocal(bad); err == nil {
			t.Errorf("ReadLocal(%q) succeeded", bad)
		}
	}
	if s.Repo() == nil {
		t.Error("Repo() = nil")
	}
}

func TestWithLocalLock(t *testing.T) {
	s := newTestStore(t)
	ran := false
	err := s.WithLocalLock("sync.lock", 0, func() error {
		ran = true
		// The lock is taken: another holder does not get it.
		return s.WithLocalLock("sync.lock", 0, func() error { return nil })
	})
	if !ran || !errors.Is(err, ErrLocked) {
		t.Errorf("nested WithLocalLock: ran %v, err = %v", ran, err)
	}
	if err := s.WithLocalLock("sync.lock", 0, func() error { return nil }); err != nil {
		t.Errorf("the lock was not released: %v", err)
	}
	if err := s.WithLocalLock("a/b", 0, func() error { return nil }); err == nil {
		t.Error("WithLocalLock(a/b) succeeded")
	}
	if s.Dir() == "" {
		t.Error("Dir() is empty")
	}
}
