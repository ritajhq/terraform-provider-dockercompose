package provider

import (
	"fmt"

	"github.com/xRizur/terraform-provider-dockercompose/internal/docker"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// composeFileHasWatchEntries reports whether any service in the compose file
// defines at least one develop.watch entry.
func composeFileHasWatchEntries(cf *docker.ComposeFile) bool {
	for _, svc := range cf.Services {
		if svc.Develop != nil && len(svc.Develop.Watch) > 0 {
			return true
		}
	}
	return false
}

// startOrPlainUp starts the stack via the detached watcher when watch is requested
// and at least one service defines develop.watch entries; otherwise it falls back
// to a plain `compose up -d`. profiles, if non-empty, activates the given Compose
// profiles via --profile flags on either path. Returns the watcher PID (0 if not started).
func startOrPlainUp(client *docker.DockerClient, stackName, composeFilePath string, watchRequested bool, cf *docker.ComposeFile, profiles []string) (int, error) {
	if watchRequested && composeFileHasWatchEntries(cf) {
		pid, err := client.ComposeUpWatchProfiles(stackName, composeFilePath, profiles)
		if err != nil {
			return 0, fmt.Errorf("error starting watcher: %s", err)
		}
		return pid, nil
	}

	// watch = true but no service defines develop.watch: fall back to plain up -d silently.
	if _, err := client.ComposeUpProfiles(stackName, composeFilePath, profiles); err != nil {
		return 0, fmt.Errorf("error starting stack: %s", err)
	}
	return 0, nil
}

// setWatchPID sets the watch_pid attribute on the resource, using 0 to represent "not running".
func setWatchPID(d *schema.ResourceData, pid int) error {
	if err := d.Set("watch_pid", pid); err != nil {
		return fmt.Errorf("error setting watch_pid: %s", err)
	}
	return nil
}

// reconcileWatchOnRead checks whether the recorded watcher PID (state or pidfile) is
// still alive and clears watch_pid if it has died, so drift is visible without erroring.
func reconcileWatchOnRead(d *schema.ResourceData, composeFilePath string) error {
	pid := d.Get("watch_pid").(int)
	if pid == 0 {
		// Nothing in state; fall back to the pidfile in case it's out of sync.
		filePid, err := docker.ReadWatchPID(composeFilePath)
		if err == nil && filePid != 0 {
			pid = filePid
		}
	}

	if pid == 0 {
		return nil
	}

	if docker.IsProcessAlive(pid) {
		return setWatchPID(d, pid)
	}

	// Recorded PID is no longer running: clear state so drift is visible.
	if err := docker.RemoveWatchPidFile(composeFilePath); err != nil {
		return fmt.Errorf("error removing stale watch pidfile: %s", err)
	}
	return setWatchPID(d, 0)
}

// reconcileWatchOnUpdate converges the watcher process to match the desired watch state:
// killing it if watch is no longer requested (or no watch entries remain), spawning it if
// requested and not running, or always killing and respawning it if requested and already
// running, per the "keep behavior simple and predictable" update contract. profiles, if
// non-empty, activates the given Compose profiles via --profile flags on the final up step.
func reconcileWatchOnUpdate(client *docker.DockerClient, stackName, composeFilePath string, watchRequested bool, cf *docker.ComposeFile, profiles []string) (int, error) {
	existingPid, err := docker.ReadWatchPID(composeFilePath)
	if err != nil {
		return 0, fmt.Errorf("error reading watch pidfile: %s", err)
	}
	watcherRunning := existingPid != 0 && docker.IsProcessAlive(existingPid)

	shouldWatch := watchRequested && composeFileHasWatchEntries(cf)

	if watcherRunning {
		if err := docker.KillWatchProcess(existingPid); err != nil {
			return 0, fmt.Errorf("error stopping existing watcher: %s", err)
		}
		if err := docker.RemoveWatchPidFile(composeFilePath); err != nil {
			return 0, fmt.Errorf("error removing watch pidfile: %s", err)
		}
	}

	if shouldWatch {
		pid, err := client.ComposeUpWatchProfiles(stackName, composeFilePath, profiles)
		if err != nil {
			return 0, fmt.Errorf("error starting watcher: %s", err)
		}
		return pid, nil
	}

	if _, err := client.ComposeUpProfiles(stackName, composeFilePath, profiles); err != nil {
		return 0, fmt.Errorf("error starting stack: %s", err)
	}
	return 0, nil
}

// stopWatchOnDelete kills any running watcher recorded for composeFilePath before destroy.
// If the recorded PID is already dead, it proceeds without failing destroy.
func stopWatchOnDelete(composeFilePath string) error {
	pid, err := docker.ReadWatchPID(composeFilePath)
	if err != nil {
		return fmt.Errorf("error reading watch pidfile: %s", err)
	}
	if pid == 0 {
		return nil
	}

	if !docker.IsProcessAlive(pid) {
		// Already dead: nothing to kill, proceed with destroy.
		return docker.RemoveWatchPidFile(composeFilePath)
	}

	if err := docker.KillWatchProcess(pid); err != nil {
		return fmt.Errorf("error stopping watcher (pid %d): %s", pid, err)
	}
	return docker.RemoveWatchPidFile(composeFilePath)
}
