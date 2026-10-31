package gitrepotest

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// LockObjects makes the objects directory of the bare repository dir, and
// every directory in it, read-only, so that a push to it is denied and
// gitrepo.Writable is false. unlock gives the write permission back; the
// cleanup of the test calls it too, so that TempDir can remove the
// directory. The test is skipped where a directory cannot be made read-only
// to the user who runs it: on Windows, and as root.
func LockObjects(t testing.TB, dir string) (unlock func()) {
	t.Helper()
	if runtime.GOOS == "windows" || os.Getuid() == 0 {
		t.Skip("no read-only directories here")
	}
	var dirs []string
	_ = filepath.WalkDir(filepath.Join(dir, "objects"), func(p string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			dirs = append(dirs, p)
		}
		return nil
	})
	for _, p := range dirs {
		if err := os.Chmod(p, 0o500); err != nil { //nolint:gosec // G302: a read-only directory is the point of this helper
			t.Fatal(err)
		}
	}
	unlock = func() {
		for _, p := range dirs {
			_ = os.Chmod(p, 0o700) //nolint:gosec // G302: give the write permission back so that TempDir can remove the directory
		}
	}
	t.Cleanup(unlock)
	return unlock
}
