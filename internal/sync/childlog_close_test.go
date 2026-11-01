package sync

import (
	"os"
	"path/filepath"
	"testing"
)

// A log that could not be closed is not released: it takes lines as before.
func TestBoundedLogReleaseThatFailsKeepsWriting(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "sync.log")
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close() // a second Close fails
	w := &BoundedLog{w: f, path: p, own: f}
	if err := w.Release(); err == nil {
		t.Fatal("Release of a log that cannot be closed did not fail")
	}
	if w.released {
		t.Error("the log is released though the close failed")
	}
	if _, err := w.Write([]byte("x\n")); err == nil {
		t.Error("the line was held, not written to the log")
	}
}
