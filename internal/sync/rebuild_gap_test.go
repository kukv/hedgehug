package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
)

// A write to .hedgehug/local of the old store between the last look and the
// rename is carried over after the rename; the inbox's lines are joined with
// no line twice.
func TestSwapCarriesLocalWritesOfTheGap(t *testing.T) {
	c := newCluster(t, 2)
	sc := newRewriteScene(t, c, func(*rewriteScene) {})
	old := c.stores[1]
	line := `{"ulid":"` + model.NewULID().String() + `","kind":"watch","task":"` + sc.keep.ULID.String() + `","commit":"x","author":"","to":"github.com/u1","time":"2026-11-02T00:00:00Z"}` + "\n"
	prev := beforeRename
	beforeRename = func() {
		if err := old.WriteLocal("late.json", []byte("late\n")); err != nil {
			t.Error(err)
		}
		if err := old.WriteLocal("inbox.jsonl", []byte(line)); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() { beforeRename = prev })
	c.swap(sc.res)
	s := c.stores[1]
	if b, err := s.ReadLocal("late.json"); err != nil || string(b) != "late\n" {
		t.Errorf("late.json = %q, %v", b, err)
	}
	b, err := s.ReadLocal("inbox.jsonl")
	if err != nil || strings.Count(string(b), line) != 1 || !strings.Contains(string(b), `"rewrite_applied"`) {
		t.Errorf("inbox.jsonl = %q, %v", b, err)
	}
	if left, err := FindLeftover(s.Dir()); err != nil || !left.None() {
		t.Errorf("leftover = %+v, %v", left, err)
	}
}

// Release closes the log file and keeps what is written meanwhile; Reopen
// opens the log at the path, in the directory that is there now, and writes
// it.
func TestBoundedLogReleaseAndReopen(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "s")
	p := filepath.Join(dir, ".hedgehug", "local", "sync.log")
	f, err := openLog(p)
	if err != nil {
		t.Fatal(err)
	}
	w := &BoundedLog{w: f, path: p, own: f}
	if _, err := w.Write([]byte("one\n")); err != nil {
		t.Fatal(err)
	}
	if err := w.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("x")); err == nil {
		t.Error("the file is still open")
	}
	if _, err := w.Write([]byte("two\n")); err != nil {
		t.Fatal(err)
	}
	// the directory is replaced
	if err := os.Rename(dir, dir+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := w.Reopen(); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("three\n")); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil || string(b) != "two\nthree\n" {
		t.Errorf("new log = %q, %v", b, err)
	}
	if err := w.own.Close(); err != nil {
		t.Fatal(err)
	}
}
