package gitrepo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVerify(t *testing.T) {
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Verify(); err != nil {
		t.Errorf("empty repo: %v", err)
	}
	writeFile(t, dir, "d/a.md", "some content that is long enough to compress")
	if _, err := r.Commit(Commit{Paths: []string{"d/a.md"}, Subject: "one", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := r.Verify(); err != nil {
		t.Fatalf("healthy repo: %v", err)
	}

	th, err := r.HeadTree()
	if err != nil {
		t.Fatal(err)
	}
	tr, err := r.tree(th)
	if err != nil {
		t.Fatal(err)
	}
	f, err := tr.File("d/a.md")
	if err != nil {
		t.Fatal(err)
	}
	blob := f.Hash.String()
	obj := filepath.Join(dir, ".git", "objects", blob[:2], blob[2:])
	if err := os.Chmod(obj, 0o600); err != nil { // git writes objects read-only
		t.Fatal(err)
	}
	if err := os.Truncate(obj, 5); err != nil {
		t.Fatal(err)
	}
	r2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	err = r2.Verify()
	if err == nil {
		t.Fatal("Verify of corrupted repo = nil")
	}
	if !strings.Contains(err.Error(), blob) {
		t.Errorf("err = %v, want hash %s", err, blob)
	}
}
