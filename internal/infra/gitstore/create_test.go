package gitstore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

var t0 = time.Date(2026, 11, 12, 9, 0, 0, 0, time.FixedZone("JST", 9*3600))

func newStore(t *testing.T) (port.StoreID, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "store")
	if err := Create(dir, "01STORE0000000000000000001", t0); err != nil {
		t.Fatal(err)
	}
	s := New(apptest.NewClock(t0), apptest.NewIDs())
	id, err := s.Attach(dir)
	if err != nil {
		t.Fatal(err)
	}
	return id, dir
}

func TestCreateWritesStoreYAMLInOneCommit(t *testing.T) {
	id, dir := newStore(t)
	if id != "01STORE0000000000000000001" {
		t.Errorf("id = %s", id)
	}
	b, err := os.ReadFile(filepath.Join(dir, ".hedgehug", "store.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	want := "store: 01STORE0000000000000000001\nformat: 1\nretention_days: 30\n"
	if string(b) != want {
		t.Errorf("store.yaml =\n%s\nwant\n%s", b, want)
	}
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	head, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	c, err := repo.CommitObject(head.Hash())
	if err != nil {
		t.Fatal(err)
	}
	// 最初のコミットは保存ではないので Hedgehug-Op を持たない(保存形式「.hedgehug/store.yaml」)。
	if c.Author.Name != "hedgehug[bot]" || c.Author.Email != "bot@hedgehug.invalid" || c.NumParents() != 0 || c.Message != "Store を作成\n" {
		t.Errorf("commit = %+v", c)
	}
}

func TestCreateUsesMainBranch(t *testing.T) {
	_, dir := newStore(t)
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := repo.Storer.Reference(plumbing.HEAD)
	if err != nil {
		t.Fatal(err)
	}
	if ref.Type() != plumbing.SymbolicReference || ref.Target() != plumbing.NewBranchReferenceName("main") {
		t.Errorf("HEAD = %s", ref)
	}
	head, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	if head.Name() != plumbing.NewBranchReferenceName("main") {
		t.Errorf("head = %s", head.Name())
	}
}

func TestCreateRefusesNonEmptyDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Create(dir, "01STORE0000000000000000001", t0); err == nil {
		t.Error("Create on a non-empty dir succeeded")
	}
}

func TestAttachRefuses(t *testing.T) {
	s := New(apptest.NewClock(t0), apptest.NewIDs())
	if _, err := s.Attach(t.TempDir()); !errors.Is(err, ErrNotStore) {
		t.Errorf("plain dir: %v", err)
	}
	_, dir := newStore(t)
	if err := os.WriteFile(filepath.Join(dir, ".hedgehug", "store.yaml"), []byte("store: X\nformat: 2\nretention_days: 30\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Attach(dir); !errors.Is(err, ErrNewerFormat) {
		t.Errorf("newer format: %v", err)
	}
}

func TestUnknownStore(t *testing.T) {
	s := New(apptest.NewClock(t0), apptest.NewIDs())
	if _, err := s.dir("nope"); !errors.Is(err, port.ErrStoreNotFound) {
		t.Errorf("dir: %v", err)
	}
}

func TestLockIsExclusive(t *testing.T) {
	_, dir := newStore(t)
	unlock, err := lock(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan struct{})
	go func() {
		u, err := lock(dir)
		if err == nil {
			u()
		}
		close(got)
	}()
	select {
	case <-got:
		t.Fatal("second lock was taken while the first was held")
	case <-time.After(100 * time.Millisecond):
	}
	unlock()
	<-got
}
