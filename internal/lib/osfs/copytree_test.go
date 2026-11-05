package osfs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyTree(t *testing.T) {
	t.Parallel()
	src := filepath.Join(t.TempDir(), "src")
	dst := src + ".copy"
	if err := os.MkdirAll(filepath.Join(src, "a", "b"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a", "b", "f"), []byte("x"), 0o440); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a", "skip"), []byte("y"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("b/f", filepath.Join(src, "a", "link")); err != nil {
		t.Fatal(err)
	}
	if err := CopyTree(src, dst, func(rel string) bool { return rel == "a/skip" }); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dst, "a", "b", "f"))
	if err != nil || info.Mode().Perm() != 0o440 {
		t.Fatalf("f = %v, %v", info, err)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "a", "b", "f")); string(b) != "x" {
		t.Errorf("f = %q", b)
	}
	if l, err := os.Readlink(filepath.Join(dst, "a", "link")); err != nil || l != "b/f" {
		t.Errorf("link = %q, %v", l, err)
	}
	if _, err := os.Stat(filepath.Join(dst, "a", "skip")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("skip = %v", err)
	}
}
