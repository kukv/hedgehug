package store

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

func TestCreateAndOpen(t *testing.T) {
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
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "x"}, nil, &model.Workspace{ULID: model.NewULID(), Name: "x", KeyPrefix: "TST"}, testAuthor); err == nil {
		t.Error("Create into a non-empty directory succeeded")
	}
}

func TestOpenNotAStore(t *testing.T) {
	if _, err := Open(t.TempDir()); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestCreateUsesTheGivenSchema(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "store")
	s, err := Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "x"}, schema.Dev(), &model.Workspace{ULID: model.NewULID(), Name: "x", KeyPrefix: "TST"}, testAuthor)
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
	meta := &model.StoreMeta{ULID: model.NewULID(), Name: "x"}
	if _, err := Create(filepath.Join(t.TempDir(), "store"), meta, nil, &model.Workspace{ULID: model.NewULID(), Name: "x", KeyPrefix: "TST"}, testAuthor); err != nil {
		t.Fatal(err)
	}
	if meta.Format != 0 {
		t.Errorf("meta.Format = %d, want 0: Create must not change the caller's meta", meta.Format)
	}
}

func TestCreateCleansUpOnFailure(t *testing.T) {
	boom := errors.New("commit failed")
	newStoreHook = func(s *Store) { s.commit = func(gitrepo.Commit) error { return boom } }
	t.Cleanup(func() { newStoreHook = func(*Store) {} })

	t.Run("a directory Create made is removed", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "store")
		if _, err := Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "x"}, nil, &model.Workspace{ULID: model.NewULID(), Name: "x", KeyPrefix: "TST"}, testAuthor); !errors.Is(err, boom) {
			t.Fatalf("err = %v, want %v", err, boom)
		}
		if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Stat(dir) err = %v, want ErrNotExist", err)
		}
	})
	t.Run("an empty directory is emptied again", func(t *testing.T) {
		dir := t.TempDir()
		if _, err := Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "x"}, nil, &model.Workspace{ULID: model.NewULID(), Name: "x", KeyPrefix: "TST"}, testAuthor); !errors.Is(err, boom) {
			t.Fatalf("err = %v, want %v", err, boom)
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
	dir := filepath.Join(t.TempDir(), "store")
	newStoreHook = func(s *Store) { s.commit = func(gitrepo.Commit) error { return errors.New("boom") } }
	if _, err := Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "x"}, nil, &model.Workspace{ULID: model.NewULID(), Name: "x", KeyPrefix: "TST"}, testAuthor); err == nil {
		t.Fatal("Create with a failing commit succeeded")
	}
	newStoreHook = func(*Store) {}
	if _, err := Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "x"}, nil, &model.Workspace{ULID: model.NewULID(), Name: "x", KeyPrefix: "TST"}, testAuthor); err != nil {
		t.Errorf("retry: %v", err)
	}
}

func TestCreateRefusesBadWorkspace(t *testing.T) {
	meta := &model.StoreMeta{ULID: model.NewULID(), Name: "x"}
	if _, err := Create(filepath.Join(t.TempDir(), "a"), meta, nil, nil, testAuthor); err == nil {
		t.Error("nil workspace is accepted")
	}
	ws := &model.Workspace{ULID: model.NewULID(), Name: "x", KeyPrefix: "x"}
	if _, err := Create(filepath.Join(t.TempDir(), "b"), meta, nil, ws, testAuthor); err == nil {
		t.Error("bad prefix is accepted")
	}
}
