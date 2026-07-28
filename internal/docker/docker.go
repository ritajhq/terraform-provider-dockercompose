package docker

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// WatchLogFileName is the name of the log file the detached watcher's stdout/stderr are redirected to.
const WatchLogFileName = "watch.log"

// WatchPidFileName is the name of the pidfile written for the detached watcher process.
const WatchPidFileName = "watch.pid"

// DockerClient wraps docker compose CLI commands with remote host support.
type DockerClient struct {
	Host             string
	Binary           string
	ProjectDirectory string

	// DefaultWatch and DefaultActiveProfiles are the provider-level defaults for the
	// dockercompose_stack resource's watch and active_profiles attributes. Resources
	// fall back to these when the corresponding field isn't explicitly set.
	DefaultWatch          bool
	DefaultActiveProfiles []string
}

// Version returns the docker compose version details.
func (c *DockerClient) Version() (string, error) {
	return c.compose("", "", "version")
}

// ComposeUp runs `docker compose up -d --remove-orphans`.
func (c *DockerClient) ComposeUp(projectName, composeFile string) (string, error) {
	return c.compose(projectName, composeFile, "up", "-d", "--remove-orphans")
}

// ComposeUpProfiles runs `docker compose up -d --remove-orphans` with the given
// profiles activated via repeated --profile flags.
func (c *DockerClient) ComposeUpProfiles(projectName, composeFile string, profiles []string) (string, error) {
	args := append(profileArgs(profiles), "up", "-d", "--remove-orphans")
	return c.compose(projectName, composeFile, args...)
}

// ComposeStop runs `docker compose stop <services...>`, stopping the given services
// without removing their containers.
func (c *DockerClient) ComposeStop(projectName, composeFile string, services []string) (string, error) {
	if len(services) == 0 {
		return "", nil
	}
	args := append([]string{"stop"}, services...)
	return c.compose(projectName, composeFile, args...)
}

// ComposePSServicesProfiles returns the list of running service names, restricted
// to the given active profiles (services outside those profiles are excluded).
func (c *DockerClient) ComposePSServicesProfiles(projectName, composeFile string, profiles []string) (string, error) {
	args := append(profileArgs(profiles), "ps", "--services")
	return c.compose(projectName, composeFile, args...)
}

// ComposePSJSONProfiles returns container status as JSON, restricted to the given
// active profiles.
func (c *DockerClient) ComposePSJSONProfiles(projectName, composeFile string, profiles []string) (string, error) {
	args := append(profileArgs(profiles), "ps", "--format", "json", "-a")
	return c.compose(projectName, composeFile, args...)
}

// profileArgs builds repeated `--profile <name>` flags for the given profiles.
func profileArgs(profiles []string) []string {
	args := make([]string, 0, len(profiles)*2)
	for _, p := range profiles {
		args = append(args, "--profile", p)
	}
	return args
}

// ComposeUpWatch launches `docker compose -p <projectName> up -d --watch` as a detached
// background process so it survives the provider process exiting. Stdout/stderr are
// redirected to watch.log next to composeFile, and the PID is written to watch.pid.
// Returns the PID of the spawned watcher.
func (c *DockerClient) ComposeUpWatch(projectName, composeFile string) (int, error) {
	return c.composeUpWatch(projectName, composeFile, nil)
}

// ComposeUpWatchProfiles is ComposeUpWatch with the given profiles activated via
// repeated --profile flags.
func (c *DockerClient) ComposeUpWatchProfiles(projectName, composeFile string, profiles []string) (int, error) {
	return c.composeUpWatch(projectName, composeFile, profiles)
}

func (c *DockerClient) composeUpWatch(projectName, composeFile string, profiles []string) (int, error) {
	binary := c.Binary
	if binary == "" {
		binary = "docker"
	}

	cmdArgs := []string{"compose"}
	if projectName != "" {
		cmdArgs = append(cmdArgs, "-p", projectName)
	}
	if composeFile != "" {
		cmdArgs = append(cmdArgs, "-f", composeFile)
	}
	cmdArgs = append(cmdArgs, profileArgs(profiles)...)
	cmdArgs = append(cmdArgs, "up", "-d", "--watch")

	cmd := exec.Command(binary, cmdArgs...)

	cmd.Env = os.Environ()
	if c.Host != "" {
		cmd.Env = append(cmd.Env, "DOCKER_HOST="+c.Host)
	}

	dir := filepath.Dir(composeFile)
	cmd.Dir = dir

	logFile, err := os.OpenFile(filepath.Join(dir, WatchLogFileName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return 0, fmt.Errorf("error opening watch log file: %s", err)
	}
	defer logFile.Close()

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	detachProcess(cmd)

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("docker compose up --watch: %s", err)
	}

	pid := cmd.Process.Pid
	if err := writePidFile(dir, pid); err != nil {
		return pid, fmt.Errorf("error writing watch pidfile: %s", err)
	}

	// Detach from the child so it isn't reaped/waited on by this process.
	if err := cmd.Process.Release(); err != nil {
		return pid, fmt.Errorf("error releasing watch process: %s", err)
	}

	return pid, nil
}

// WatchPidFilePath returns the path to the watch pidfile alongside composeFile.
func WatchPidFilePath(composeFile string) string {
	return filepath.Join(filepath.Dir(composeFile), WatchPidFileName)
}

// WatchLogFilePath returns the path to the watch log file alongside composeFile.
func WatchLogFilePath(composeFile string) string {
	return filepath.Join(filepath.Dir(composeFile), WatchLogFileName)
}

// writePidFile writes the given PID to the watch pidfile in dir.
func writePidFile(dir string, pid int) error {
	return os.WriteFile(filepath.Join(dir, WatchPidFileName), []byte(strconv.Itoa(pid)), 0644)
}

// ReadWatchPID reads the watcher PID recorded for composeFile, if any.
// Returns 0, nil if no pidfile exists.
func ReadWatchPID(composeFile string) (int, error) {
	data, err := os.ReadFile(WatchPidFilePath(composeFile))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid watch pidfile contents: %s", err)
	}
	return pid, nil
}

// RemoveWatchPidFile deletes the pidfile for composeFile, if present.
func RemoveWatchPidFile(composeFile string) error {
	err := os.Remove(WatchPidFilePath(composeFile))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ComposeDown runs `docker compose down`, optionally removing volumes.
func (c *DockerClient) ComposeDown(projectName, composeFile string, removeVolumes bool) (string, error) {
	if removeVolumes {
		return c.compose(projectName, composeFile, "down", "-v")
	}
	return c.compose(projectName, composeFile, "down")
}

// ComposePSJSON returns container status as JSON.
func (c *DockerClient) ComposePSJSON(projectName, composeFile string) (string, error) {
	return c.compose(projectName, composeFile, "ps", "--format", "json", "-a")
}

// ComposePSServices returns the list of running service names.
func (c *DockerClient) ComposePSServices(projectName, composeFile string) (string, error) {
	return c.compose(projectName, composeFile, "ps", "--services")
}

// ComposeConfig validates and outputs the resolved compose config.
func (c *DockerClient) ComposeConfig(projectName, composeFile string) (string, error) {
	return c.compose(projectName, composeFile, "config")
}

// ProjectDir returns the base directory for a stack's compose files.
func (c *DockerClient) ProjectDir(stackName string) string {
	return filepath.Join(c.ProjectDirectory, stackName)
}

// ComposeFilePath returns the default compose file path for a stack.
func (c *DockerClient) ComposeFilePath(stackName string) string {
	return filepath.Join(c.ProjectDir(stackName), "docker-compose.yml")
}

// DockerInspect runs `docker inspect` on one or more containers and returns the JSON output.
func (c *DockerClient) DockerInspect(containerIDs ...string) (string, error) {
	binary := c.Binary
	if binary == "" {
		binary = "docker"
	}

	args := append([]string{"inspect"}, containerIDs...)
	cmd := exec.Command(binary, args...)

	cmd.Env = os.Environ()
	if c.Host != "" {
		cmd.Env = append(cmd.Env, "DOCKER_HOST="+c.Host)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker inspect: %s\n%s", err, strings.TrimSpace(stderr.String()))
	}

	return strings.TrimSpace(stdout.String()), nil
}

// compose executes a docker compose command with project isolation and remote host support.
func (c *DockerClient) compose(projectName, composeFile string, args ...string) (string, error) {
	binary := c.Binary
	if binary == "" {
		binary = "docker"
	}

	cmdArgs := []string{"compose"}
	if projectName != "" {
		cmdArgs = append(cmdArgs, "-p", projectName)
	}
	if composeFile != "" {
		cmdArgs = append(cmdArgs, "-f", composeFile)
	}
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.Command(binary, cmdArgs...)

	// Inherit environment and set DOCKER_HOST if configured
	cmd.Env = os.Environ()
	if c.Host != "" {
		cmd.Env = append(cmd.Env, "DOCKER_HOST="+c.Host)
	}

	// Set working directory for relative path resolution
	if composeFile != "" {
		cmd.Dir = filepath.Dir(composeFile)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker compose %s: %s\n%s",
			strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}

	return strings.TrimSpace(stdout.String()), nil
}
