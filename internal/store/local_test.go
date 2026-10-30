package store

import (
	"errors"
	"testing"
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
	if _, err := readFile(s.path(".hedgehug/local/index")); err != nil {
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
