package atomicfile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "tasks", "A", "B", "x.md")
	if err := WriteFile(p, []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(p, []byte("two")); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "two" {
		t.Errorf("content = %q, want two", got)
	}
	entries, err := os.ReadDir(filepath.Dir(p))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("entries = %v, want only x.md (no temporary files)", entries)
	}
}

func TestReadFileNotExist(t *testing.T) {
	_, err := ReadFile(filepath.Join(t.TempDir(), "nope.md"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want fs.ErrNotExist", err)
	}
}
