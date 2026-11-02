package sync

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

// When the log is the standard error, Release points it at the null device
// and Reopen puts the log back and closes the handle of the null device.
func TestBoundedLogReopenClosesTheNullDevice(t *testing.T) {
	// 直列: 標準エラー(fd 2)
	saved, err := unix.Dup(2)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = unix.Dup2(saved, 2)
		_ = unix.Close(saved)
	})
	dir := filepath.Join(t.TempDir(), "s")
	p := filepath.Join(dir, ".hedgehug", "local", "sync.log")
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	b := &BoundedLog{w: os.Stderr, path: p, own: os.Stderr}
	if err := b.Release(); err != nil {
		t.Fatal(err)
	}
	if b.nul == nil {
		t.Fatal("Release opened no null device")
	}
	nul := b.nul
	if err := b.Reopen(); err != nil {
		t.Fatal(err)
	}
	if b.nul != nil {
		t.Error("the null device is still held")
	}
	if _, err := nul.Stat(); err == nil {
		t.Error("the handle of the null device is still open")
	}
	if b.own != nil {
		_ = b.own.Close()
	}
}

// A run that ends while the log is released closes the null device in Close.
func TestBoundedLogCloseClosesTheNullDevice(t *testing.T) {
	// 直列: 標準エラー(fd 2)
	saved, err := unix.Dup(2)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = unix.Dup2(saved, 2)
		_ = unix.Close(saved)
	})
	b := &BoundedLog{w: os.Stderr, path: filepath.Join(t.TempDir(), "sync.log"), own: os.Stderr}
	if err := b.Release(); err != nil {
		t.Fatal(err)
	}
	nul := b.nul
	if nul == nil {
		t.Fatal("Release opened no null device")
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if b.nul != nil {
		t.Error("the null device is still held")
	}
	if _, err := nul.Stat(); err == nil {
		t.Error("the handle of the null device is still open")
	}
	if err := b.Close(); err != nil {
		t.Errorf("a second Close = %v", err)
	}
}
