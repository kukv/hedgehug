package gitrepo

import "testing"

func TestClose(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, _ := Init(dir)
	h := commitHead(t, r, dir, map[string]string{"a.md": "a\n"})
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	r2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := r2.ReadAt(h, "a.md"); err != nil || string(got) != "a\n" {
		t.Errorf("a.md after reopen = %q, %v", got, err)
	}
}
