package sync

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/store"
)

// A sync takes a broken access.json as absent: it warns, rewrites the file,
// and goes on (and pushes).
func TestRunWithBrokenAccessFile(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	c.addTask(0, "local")
	s := c.stores[0]
	if err := s.WriteLocal(accessFile, []byte("{not json")); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadAccess(s); !errors.Is(err, ErrBrokenAccess) {
		t.Fatalf("ReadAccess = %v; want ErrBrokenAccess", err)
	}
	res := c.sync(0)
	want := filepath.Join(s.Dir(), store.LocalDir, accessFile) + " is broken; judging access again"
	if !slices.Contains(res.Warnings, want) {
		t.Errorf("warnings = %q; want %q", res.Warnings, want)
	}
	if len(res.Pushed) == 0 {
		t.Errorf("pushed = %v; want the sync to push", res.Pushed)
	}
	if a, err := ReadAccess(s); err != nil || a.ReadOnly {
		t.Errorf("access after the sync = %+v, %v; want a valid, writable file", a, err)
	}
	if again := c.sync(0); slices.ContainsFunc(again.Warnings, func(w string) bool { return strings.Contains(w, "broken") }) {
		t.Errorf("warnings of the next sync = %q; want none about the file", again.Warnings)
	}
}

func TestCheckAccessWithBrokenAccessFile(t *testing.T) {
	t.Parallel()
	s := newLoneStore(t)
	if err := s.WriteLocal(accessFile, []byte("[")); err != nil {
		t.Fatal(err)
	}
	a, warns, err := CheckAccess(s, Options{}, time.Now())
	if err != nil || a.ReadOnly || len(warns) != 1 || !strings.Contains(warns[0], "is broken; judging access again") {
		t.Errorf("CheckAccess = %+v, %q, %v", a, warns, err)
	}
	if _, err := ReadAccess(s); err != nil {
		t.Errorf("the file was not rewritten: %v", err)
	}
}
