package osfs

import (
	"os"
	"os/exec"
	"path/filepath"
)

// Command is exec.Command: name run with args, which the caller sets up and
// runs.
func Command(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...) //nolint:gosec // G204: callers choose the program and its arguments
}

// LookPath finds the executable called name in the directories of path, a
// list in the form of the PATH variable. It is exec.LookPath with the list
// given instead of read from the process.
func LookPath(name, path string) (string, error) {
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			dir = "." // as in a shell, an empty entry is the current directory
		}
		p := filepath.Join(dir, name)
		if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() && fi.Mode().Perm()&0o111 != 0 {
			return p, nil
		}
	}
	return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
}
