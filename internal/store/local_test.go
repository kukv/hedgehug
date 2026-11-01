package store

import (
	"errors"
	"io/fs"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/atomicfile"
)

func TestLocalFiles(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	if _, err := s.ReadLocal("index"); !errors.Is(err, ErrNotFound) || !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing file: err = %v, want ErrNotFound and fs.ErrNotExist", err)
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
	for _, bad := range []string{"", ".", "..", "a/..", "../a", "a/./b", "/a", "a/", "a//b", `a\b`} {
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
	t.Parallel()
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
	if err := s.WithLocalLock("../b", 0, func() error { return nil }); err == nil {
		t.Error("WithLocalLock(../b) succeeded")
	}
	if s.Dir() == "" {
		t.Error("Dir() is empty")
	}
}

func TestLocalDirectory(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	if names, err := s.LocalFiles("records"); err != nil || len(names) != 0 {
		t.Errorf("missing directory: %v, %v", names, err)
	}
	for _, n := range []string{"records/b.json", "records/a.json", "records/sub/c.json", "records/.a.json.tmp-1"} {
		if err := s.WriteLocal(n, []byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	names, err := s.LocalFiles("records")
	if err != nil || !slices.Equal(names, []string{"a.json", "b.json"}) {
		t.Errorf("LocalFiles = %v, %v", names, err)
	}
	if err := s.RemoveLocal("records/a.json"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadLocal("records/a.json"); !errors.Is(err, ErrNotFound) {
		t.Errorf("removed file: err = %v", err)
	}
	if err := s.RemoveLocal("records/a.json"); err != nil {
		t.Errorf("removing a missing file: %v", err)
	}
	if _, err := s.LocalFiles(".."); err == nil {
		t.Error("LocalFiles(..) succeeded")
	}
	if err := s.RemoveLocal("../x"); err == nil {
		t.Error("RemoveLocal(../x) succeeded")
	}
}
