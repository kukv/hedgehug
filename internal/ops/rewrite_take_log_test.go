package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// When the swap fails and leaves no store at the path, the child's log is
// opened in the directory that was left, and the error is written there.
func TestBackgroundSyncLogsTheFailedSwapWhereTheStoreIs(t *testing.T) {
	f := newTakeFx(t)
	path := f.path(t)
	f.clone.appendConfig("sync:\n  push_interval: 1ms\n")
	ss := f.clone.ForStore("me")
	log, err := os.OpenFile(filepath.Join(path, ".hedgehug", "local", "sync.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = log.Close() }()
	before := path + ".before-rewrite-20260101T000000.000000000"
	ss.BeforeSwap = func(p string) {
		if err := os.Rename(p, before); err != nil {
			t.Error(err)
		}
	}
	if err := ss.SyncBackground(log); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("a directory was made at the path")
	}
	b, err := os.ReadFile(filepath.Join(before, ".hedgehug", "local", "sync.log"))
	if err != nil || !strings.Contains(string(b), "sync me: ") {
		t.Errorf("sync.log of the old store = %q, %v", b, err)
	}
}
