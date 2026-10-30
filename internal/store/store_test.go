package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
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
	if o.Meta().ID != s.Meta().ID || o.Meta().Name != "test" {
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
	if _, err := Create(dir, &model.StoreMeta{ID: model.NewID(), Name: "x"}, testAuthor); err == nil {
		t.Error("Create into a non-empty directory succeeded")
	}
}

func TestOpenNotAStore(t *testing.T) {
	if _, err := Open(t.TempDir()); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
