package gitrepo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
)

func TestMergeHead(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := r.MergeHead(); err != nil || ok {
		t.Fatalf("no merge: ok %v, err %v", ok, err)
	}
	want := "0123456789abcdef0123456789abcdef01234567"
	head := filepath.Join(dir, ".git", "MERGE_HEAD")
	if err := os.WriteFile(head, []byte(want+"\n"+want+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if h, ok, err := r.MergeHead(); err != nil || !ok || h != plumbing.NewHash(want) {
		t.Errorf("MergeHead = %s, %v, %v", h, ok, err)
	}
	if err := os.WriteFile(head, []byte("nonsense\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.MergeHead(); err == nil {
		t.Error("a MERGE_HEAD that names no commit is accepted")
	}

	// A .git file points at the directory that holds MERGE_HEAD.
	if err := os.WriteFile(head, []byte(want+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.Rename(filepath.Join(dir, ".git"), moved); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: "+moved+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if h, ok, err := r.MergeHead(); err != nil || !ok || h != plumbing.NewHash(want) {
		t.Errorf("MergeHead behind a .git file = %s, %v, %v", h, ok, err)
	}
}
