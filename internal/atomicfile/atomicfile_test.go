package atomicfile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "tasks", "A", "B", "x.md")
	if err := WriteFile(p, []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(p, []byte("two")); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "two" {
		t.Errorf("content = %q, want two", got)
	}
	entries, err := os.ReadDir(filepath.Dir(p))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("entries = %v, want only x.md (no temporary files)", entries)
	}
}

func TestReadFileNotExist(t *testing.T) {
	_, err := ReadFile(filepath.Join(t.TempDir(), "nope.md"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want fs.ErrNotExist", err)
	}
}

func TestRetryRename(t *testing.T) {
	old := renameInterval
	renameInterval = time.Millisecond
	t.Cleanup(func() { renameInterval = old })

	errBusy := errors.New("busy")
	errOther := errors.New("other")
	retryable := func(err error) bool { return errors.Is(err, errBusy) }

	for _, tc := range []struct {
		name      string
		fails     int   // calls that fail before one succeeds
		failWith  error // the error of a failing call
		wantCalls int
		wantErr   error
	}{
		{"succeeds at once", 0, nil, 1, nil},
		{"succeeds after retries", 3, errBusy, 4, nil},
		{"does not retry other errors", 5, errOther, 1, errOther},
		{"gives up", 100, errBusy, renameRetries + 1, errBusy},
	} {
		calls := 0
		rename := func(string, string) error {
			calls++
			if calls <= tc.fails {
				return tc.failWith
			}
			return nil
		}
		err := retryRename(rename, retryable, "a", "b")
		if calls != tc.wantCalls || !errors.Is(err, tc.wantErr) || (tc.wantErr == nil && err != nil) {
			t.Errorf("%s: calls = %d, err = %v; want %d, %v", tc.name, calls, err, tc.wantCalls, tc.wantErr)
		}
	}
}
