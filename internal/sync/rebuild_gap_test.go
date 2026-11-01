package sync

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// A write to .hedgehug/local of the old store between the last look and the
// rename is carried over after the rename; the inbox's lines are joined with
// no line twice.
func TestSwapCarriesLocalWritesOfTheGap(t *testing.T) {
	// 直列: パッケージ変数 beforeRename の差し替え
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

// A commit made to the old store between the check and the rename would
// vanish with the old directory: the swap finds it after the first rename,
// puts the old store back and tells the rebuild is stale.
func TestSwapFindsACommitOfTheGap(t *testing.T) {
	// 直列: パッケージ変数 beforeRename の差し替え
	c := newCluster(t, 2)
	sc := newRewriteScene(t, c, func(*rewriteScene) {})
	dir := c.stores[1].Dir()
	if err := c.stores[1].Repo().Close(); err != nil {
		t.Fatal(err)
	}
	prev := beforeRename
	beforeRename = func() {
		old, err := store.Open(dir)
		if err != nil {
			t.Error(err)
			return
		}
		defer func() { _ = old.Repo().Close() }()
		if _, err := old.Update(c.op(1, "update", sc.keep.ULID), func(tx *store.Tx) error {
			x, err := tx.Task(sc.keep.ULID)
			if err != nil {
				return err
			}
			x.Title = "edited in the gap"
			return tx.PutTask(x)
		}); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() { beforeRename = prev })
	if err := Swap(dir, sc.res.Rebuild); !errors.Is(err, ErrRebuildStale) {
		t.Fatalf("Swap = %v, want ErrRebuildStale", err)
	}
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	c.stores[1] = s
	if x := c.task(1, sc.keep.ULID); x == nil || x.Title != "edited in the gap" {
		t.Errorf("the commit of the gap is lost: %+v", x)
	}
	if left, err := FindLeftover(dir); err != nil || !left.None() {
		t.Errorf("leftover = %+v, %v", left, err)
	}
}

// Release closes the log file and keeps what is written meanwhile; Reopen
// opens the log at the path, in the directory that is there now, and writes
// it.
func TestBoundedLogReleaseAndReopen(t *testing.T) {
	t.Parallel()
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
