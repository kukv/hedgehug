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

func newStore(t *testing.T) (*Stores, port.StoreID, string) {
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
	return s, id, dir
}

func TestCreateWritesStoreYAMLInOneCommit(t *testing.T) {
	_, id, dir := newStore(t)
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
	_, _, dir := newStore(t)
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
	if _, err := os.Stat(filepath.Join(dir, ".git")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf(".git: %v", err)
	}
}

func TestCreateRefusesUnreadableDir(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	err := Create(file, "01STORE0000000000000000001", t0)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		t.Errorf("Create on a file: %v", err)
	}
}

func TestCreateCleansUpOnFailure(t *testing.T) {
	boom := errors.New("boom")
	afterInit = func() error { return boom }
	t.Cleanup(func() { afterInit = nil })

	missing := filepath.Join(t.TempDir(), "store")
	if err := Create(missing, "01STORE0000000000000000001", t0); !errors.Is(err, boom) {
		t.Fatalf("missing dir: %v", err)
	}
	if _, err := os.Stat(missing); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing dir was left: %v", err)
	}

	empty := t.TempDir()
	if err := Create(empty, "01STORE0000000000000000001", t0); !errors.Is(err, boom) {
		t.Fatalf("empty dir: %v", err)
	}
	if entries, err := os.ReadDir(empty); err != nil || len(entries) != 0 {
		t.Errorf("empty dir now has %v (%v)", entries, err)
	}

	afterInit = nil
	if err := Create(empty, "01STORE0000000000000000001", t0); err != nil {
		t.Errorf("retry: %v", err)
	}
}

func TestAttachRefusesZeroFormat(t *testing.T) {
	_, _, dir := newStore(t)
	for _, body := range []string{"store: X\nretention_days: 30\n", "store: X\nformat: 0\n", "store: [\n"} {
		if err := os.WriteFile(filepath.Join(dir, ".hedgehug", "store.yaml"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		s := New(apptest.NewClock(t0), apptest.NewIDs())
		if _, err := s.Attach(dir); !errors.Is(err, ErrNotStore) || err.Error() == ErrNotStore.Error() {
			t.Errorf("%q: %v", body, err)
		}
	}
}

func TestUnlockTwice(t *testing.T) {
	_, _, dir := newStore(t)
	unlock, err := lock(dir)
	if err != nil {
		t.Fatal(err)
	}
	unlock()
	unlock()
	again, err := lock(dir)
	if err != nil {
		t.Fatal(err)
	}
	again()
}

func TestAttachRefuses(t *testing.T) {
	s := New(apptest.NewClock(t0), apptest.NewIDs())
	if _, err := s.Attach(t.TempDir()); !errors.Is(err, ErrNotStore) {
		t.Errorf("plain dir: %v", err)
	}
	_, _, dir := newStore(t)
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
	_, _, dir := newStore(t)
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

// 保持日数は store.yaml の retention_days を読む。ない・0 以下のときは 30(A4b-6)。
func TestRetentionDaysReadsStoreYAML(t *testing.T) {
	s, id, dir := newStore(t)
	cases := []struct {
		name, body string
		want       int
	}{
		{"as written", "store: 01STORE0000000000000000001\nformat: 1\nretention_days: 7\n", 7},
		{"missing", "store: 01STORE0000000000000000001\nformat: 1\n", 30},
		{"zero", "store: 01STORE0000000000000000001\nformat: 1\nretention_days: 0\n", 30},
		{"negative", "store: 01STORE0000000000000000001\nformat: 1\nretention_days: -3\n", 30},
	}
	for _, c := range cases {
		if err := os.WriteFile(filepath.Join(dir, ".hedgehug", "store.yaml"), []byte(c.body), 0o600); err != nil {
			t.Fatal(err)
		}
		err := s.Read(t.Context(), id, func(r port.Reader) error {
			days, err := r.RetentionDays()
			if err != nil || days != c.want {
				t.Errorf("%s: RetentionDays = %d, %v; want %d", c.name, days, err, c.want)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
