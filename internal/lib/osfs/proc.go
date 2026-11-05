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
// list in the form of the PATH variable. It is exec.LookPath's unix search
// with the list given instead of read from the process. It differs in that
// it tests the permission bits for an execute bit instead of asking the
// system, and does not handle a name with a slash or Windows. As in
// exec.LookPath, a match in a relative directory (an empty entry is the
// current directory) is not trusted: it is reported as an error wrapping
// exec.ErrDot unless a later absolute entry holds the program.
func LookPath(name, path string) (string, error) {
	var dotErr error
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			dir = "."
		}
		p := filepath.Join(dir, name)
		fi, err := os.Stat(p)
		if err != nil || !fi.Mode().IsRegular() || fi.Mode().Perm()&0o111 == 0 {
			continue
		}
		if !filepath.IsAbs(p) {
			if dotErr == nil {
				dotErr = &exec.Error{Name: name, Err: exec.ErrDot}
			}
			continue
		}
		return p, nil
	}
	if dotErr != nil {
		return "", dotErr
	}
	return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
}
