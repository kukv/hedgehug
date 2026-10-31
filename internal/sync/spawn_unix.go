//go:build !windows

package sync

import (
	"os/exec"
	"syscall"
)

// spawnDetached starts name with args in a session of its own, with no
// input and no output but its stderr, which goes to the log.
func spawnDetached(name string, args []string, logPath string) error {
	log, err := openLog(logPath)
	if err != nil {
		return err
	}
	defer func() { _ = log.Close() }() // the child has its own copy

	cmd := exec.Command(name, args...) //nolint:gosec // G204: name is this executable
	cmd.Stderr = log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
