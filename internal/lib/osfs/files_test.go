package osfs

import "testing"

func TestSamePath(t *testing.T) {
	t.Parallel()
	if !SamePath("/a/b", "/a/b") {
		t.Error(`SamePath("/a/b", "/a/b") = false`)
	}
	if SamePath("/a/b", "/a/c") {
		t.Error(`SamePath("/a/b", "/a/c") = true`)
	}
}
