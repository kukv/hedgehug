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

// Live counts the repositories Close was not called on, once per repository.
func TestLiveCountsUnclosedRepos(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, _ := Init(dir)
	if got := Live(dir); got != 1 {
		t.Fatalf("Live after Init = %d, want 1", got)
	}
	r2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := Live(dir); got != 2 {
		t.Fatalf("Live after Open = %d, want 2", got)
	}
	_ = r.Close()
	_ = r.Close()
	if got := Live(dir); got != 1 {
		t.Errorf("Live after closing one twice = %d, want 1", got)
	}
	_ = r2.Close()
	if got := Live(dir); got != 0 {
		t.Errorf("Live after closing both = %d, want 0", got)
	}
}
