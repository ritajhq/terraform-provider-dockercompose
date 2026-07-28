//go:build windows

package docker

import (
	"os"
	"os/exec"
	"time"

	"golang.org/x/sys/windows"
)

// killGracePeriod is how long we wait after the initial kill attempt before
// confirming termination. Windows has no SIGTERM/SIGKILL distinction, so
// Process.Kill() is used directly.
const killGracePeriod = 5 * time.Second

// detachProcess is a no-op on Windows; Setsid is a Unix-only concept, so
// there is nothing to configure here for detachment.
func detachProcess(cmd *exec.Cmd) {}

// IsProcessAlive reports whether a process with the given PID is currently running.
func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)

	event, err := windows.WaitForSingleObject(handle, 0)
	if err != nil {
		return false
	}
	// WAIT_TIMEOUT means the process is still running (the wait didn't complete).
	return event == uint32(windows.WAIT_TIMEOUT)
}

// KillWatchProcess terminates pid. Windows lacks a graceful SIGTERM, so this
// calls Process.Kill() directly.
func KillWatchProcess(pid int) error {
	if pid <= 0 {
		return nil
	}
	if !IsProcessAlive(pid) {
		return nil
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Kill()
}
