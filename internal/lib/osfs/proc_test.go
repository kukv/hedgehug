package osfs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStartDetached(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "log")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := StartDetached("sh", []string{"-c", "echo detached >&2"}, f); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		b, err := os.ReadFile(p)
		if err == nil && strings.Contains(string(b), "detached") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the log has %q, want the child's stderr", b)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
