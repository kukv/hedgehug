package store

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
	"github.com/kukv/hedgehug/internal/lib/gitrepo/gitrepotest"
)

func TestCreateAndOpen(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	if s.Meta().Format != model.StoreFormat {
		t.Errorf("format = %d, want %d", s.Meta().Format, model.StoreFormat)
	}

	o, err := Open(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	if o.Meta().ULID != s.Meta().ULID || o.Meta().Name != "test" {
		t.Errorf("meta = %+v", o.Meta())
	}
	if _, ok := o.Schema().Status("todo"); !ok {
		t.Error("schema.yaml is not the default schema")
	}
	gi, err := os.ReadFile(filepath.Join(s.dir, ".gitignore"))
	if err != nil || string(gi) != ".hedgehug/local/\n" {
		t.Errorf(".gitignore = %q, %v", gi, err)
	}

	log := headLog(t, s.dir)
	if len(log) != 1 {
		t.Fatalf("commits = %d, want 1", len(log))
	}
	if want := "hedgehug: init\n\nHedgehug-Op: init\n"; log[0].Message != want {
		t.Errorf("message = %q, want %q", log[0].Message, want)
	}
	tree, err := log[0].Tree()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{".hedgehug/store.yaml", "schema.yaml", ".gitignore"} {
		if _, err := tree.File(p); err != nil {
			t.Errorf("%s is not in the first commit: %v", p, err)
		}
	}
}

func TestCreateRefusesNonEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "x"}, nil, &model.Workspace{ULID: model.NewULID(), Name: "x", IDPrefix: "TST"}, testAuthor, "", time.Now); err == nil {
		t.Error("Create into a non-empty directory succeeded")
	}
}

func TestOpenNotAStore(t *testing.T) {
	t.Parallel()
	if _, err := Open(t.TempDir()); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestCreateUsesTheGivenSchema(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "store")
	s, err := Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "x"}, schema.Dev(), &model.Workspace{ULID: model.NewULID(), Name: "x", IDPrefix: "TST"}, testAuthor, "", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Schema().Status("review"); !ok {
		t.Error("the store does not have the review status of schema.Dev")
	}
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s2.Schema().Status("review"); !ok {
		t.Error("schema.yaml on disk does not have the review status")
	}
}

func TestCreateLeavesMetaAlone(t *testing.T) {
	t.Parallel()
	meta := &model.StoreMeta{ULID: model.NewULID(), Name: "x"}
	if _, err := Create(filepath.Join(t.TempDir(), "store"), meta, nil, &model.Workspace{ULID: model.NewULID(), Name: "x", IDPrefix: "TST"}, testAuthor, "", time.Now); err != nil {
		t.Fatal(err)
	}
	if meta.Format != 0 {
		t.Errorf("meta.Format = %d, want 0: Create must not change the caller's meta", meta.Format)
	}
}

// busyAfterInit is the clock of a Create that fails: Create asks for the time
// once git init is done and before it writes a file, and the .git/index.lock
// made then keeps its commit out, as a git run by hand would.
func busyAfterInit(t *testing.T, dir string) func() time.Time {
	return func() time.Time {
		gitrepotest.HoldIndexLock(t, dir)
		return time.Now()
	}
}

func TestCreateCleansUpOnFailure(t *testing.T) {
	t.Parallel()
	t.Run("a directory Create made is removed", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join(t.TempDir(), "store")
		if _, err := Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "x"}, nil, &model.Workspace{ULID: model.NewULID(), Name: "x", IDPrefix: "TST"}, testAuthor, "", busyAfterInit(t, dir)); !errors.Is(err, gitrepo.ErrGitBusy) {
			t.Fatalf("err = %v, want ErrGitBusy", err)
		}
		if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Stat(dir) err = %v, want ErrNotExist", err)
		}
	})
	t.Run("an empty directory is emptied again", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if _, err := Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "x"}, nil, &model.Workspace{ULID: model.NewULID(), Name: "x", IDPrefix: "TST"}, testAuthor, "", busyAfterInit(t, dir)); !errors.Is(err, gitrepo.ErrGitBusy) {
			t.Fatalf("err = %v, want ErrGitBusy", err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Errorf("dir has %d entries after a failed Create, want 0", len(entries))
		}
	})
}

func TestCreateCanRetryAfterFailure(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "store")
	if _, err := Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "x"}, nil, &model.Workspace{ULID: model.NewULID(), Name: "x", IDPrefix: "TST"}, testAuthor, "", busyAfterInit(t, dir)); err == nil {
		t.Fatal("Create with a failing commit succeeded")
	}
	if _, err := Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "x"}, nil, &model.Workspace{ULID: model.NewULID(), Name: "x", IDPrefix: "TST"}, testAuthor, "", time.Now); err != nil {
		t.Errorf("retry: %v", err)
	}
}

func TestCreateRefusesBadWorkspace(t *testing.T) {
	t.Parallel()
	meta := &model.StoreMeta{ULID: model.NewULID(), Name: "x"}
	if _, err := Create(filepath.Join(t.TempDir(), "a"), meta, nil, nil, testAuthor, "", time.Now); err == nil {
		t.Error("nil workspace is accepted")
	}
	ws := &model.Workspace{ULID: model.NewULID(), Name: "x", IDPrefix: "x"}
	if _, err := Create(filepath.Join(t.TempDir(), "b"), meta, nil, ws, testAuthor, "", time.Now); err == nil {
		t.Error("bad prefix is accepted")
	}
}
