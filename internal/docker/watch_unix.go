//go:build !windows

package docker

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

// killGracePeriod is how long we wait after SIGTERM before escalating to SIGKILL.
const killGracePeriod = 5 * time.Second

// detachProcess configures cmd so the spawned process survives the parent exiting.
func detachProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

// IsProcessAlive reports whether a process with the given PID is currently running.
func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds; signal 0 checks for existence/permission.
	return process.Signal(syscall.Signal(0)) == nil
}

// KillWatchProcess sends SIGTERM to pid, waits up to killGracePeriod, then sends
// SIGKILL if the process is still alive.
func KillWatchProcess(pid int) error {
	if pid <= 0 {
		return nil
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		if !IsProcessAlive(pid) {
			return nil
		}
		return err
	}

	deadline := time.Now().Add(killGracePeriod)
	for time.Now().Before(deadline) {
		if !IsProcessAlive(pid) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	if !IsProcessAlive(pid) {
		return nil
	}

	return process.Signal(syscall.SIGKILL)
}
