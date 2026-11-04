package osfs

import "os/exec"

// Command is exec.Command: name run with args, which the caller sets up and
// runs.
func Command(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...) //nolint:gosec // G204: callers choose the program and its arguments
}
