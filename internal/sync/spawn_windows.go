package sync

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// spawnDetached starts name with args with no console and in a process group
// of its own, with no input and no output but its stderr, which goes to the
// log.
func spawnDetached(name string, args []string, logPath string) error {
	log, err := openLog(logPath)
	if err != nil {
		return err
	}
	defer func() { _ = log.Close() }() // the child has its own copy

	cmd := exec.Command(name, args...) //nolint:gosec // G204: name is this executable
	cmd.Stderr = log
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
