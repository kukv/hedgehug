//go:build unix

package osfs

import (
	"os"
	"os/exec"
	"syscall"
)

// StartDetached starts name with args in a session of its own, with no input
// and no output but stderr, and lets it run on after this process ends.
func StartDetached(name string, args []string, stderr *os.File) error {
	cmd := exec.Command(name, args...) //nolint:gosec // G204: callers choose the program and its arguments
	cmd.Stderr = stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// StopSignals are the signals that ask a process to end: Ctrl-C and SIGTERM.
func StopSignals() []os.Signal { return []os.Signal{os.Interrupt, syscall.SIGTERM} }
