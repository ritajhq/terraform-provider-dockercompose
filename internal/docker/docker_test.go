package docker

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// ============================================================
// Unit Tests for docker.go profile flag construction
// ============================================================

func TestProfileArgsEmpty(t *testing.T) {
	args := profileArgs(nil)
	if len(args) != 0 {
		t.Errorf("profileArgs(nil) = %v, want empty", args)
	}

	args = profileArgs([]string{})
	if len(args) != 0 {
		t.Errorf("profileArgs([]) = %v, want empty", args)
	}
}

func TestProfileArgsSingle(t *testing.T) {
	args := profileArgs([]string{"dev"})
	expected := []string{"--profile", "dev"}
	if len(args) != len(expected) {
		t.Fatalf("profileArgs([dev]) = %v, want %v", args, expected)
	}
	for i := range expected {
		if args[i] != expected[i] {
			t.Errorf("profileArgs([dev])[%d] = %q, want %q", i, args[i], expected[i])
		}
	}
}

func TestProfileArgsMultiple(t *testing.T) {
	args := profileArgs([]string{"dev", "debug"})
	expected := []string{"--profile", "dev", "--profile", "debug"}
	if len(args) != len(expected) {
		t.Fatalf("profileArgs([dev debug]) = %v, want %v", args, expected)
	}
	for i := range expected {
		if args[i] != expected[i] {
			t.Errorf("profileArgs([dev debug])[%d] = %q, want %q", i, args[i], expected[i])
		}
	}
}

// ============================================================
// Unit Tests for docker.go DockerClient path helpers
// ============================================================

func TestDockerClientProjectDir(t *testing.T) {
	client := &DockerClient{
		ProjectDirectory: "/opt/compose",
	}

	result := client.ProjectDir("myapp")
	expected := filepath.Join("/opt/compose", "myapp")
	if result != expected {
		t.Errorf("ProjectDir('myapp') = %q, want %q", result, expected)
	}
}

func TestDockerClientComposeFilePath(t *testing.T) {
	client := &DockerClient{
		ProjectDirectory: "/opt/compose",
	}

	result := client.ComposeFilePath("myapp")
	expected := filepath.Join("/opt/compose", "myapp", "docker-compose.yml")
	if result != expected {
		t.Errorf("ComposeFilePath('myapp') = %q, want %q", result, expected)
	}
}

func TestDockerClientBinaryDefault(t *testing.T) {
	// Verify that empty binary defaults correctly
	client := &DockerClient{
		Binary: "",
	}

	// We can't easily test compose() without Docker, but we can at least
	// verify ProjectDir and ComposeFilePath work with empty ProjectDirectory
	result := client.ProjectDir("test")
	if result != "test" {
		t.Errorf("ProjectDir with empty base = %q, want 'test'", result)
	}
}

// ============================================================
// Unit Tests for watch pidfile helpers
// ============================================================

func TestWatchPidFilePath(t *testing.T) {
	composeFile := filepath.Join("/opt/compose", "myapp", "docker-compose.yml")
	result := WatchPidFilePath(composeFile)
	expected := filepath.Join("/opt/compose", "myapp", "watch.pid")
	if result != expected {
		t.Errorf("WatchPidFilePath(%q) = %q, want %q", composeFile, result, expected)
	}
}

func TestWatchLogFilePath(t *testing.T) {
	composeFile := filepath.Join("/opt/compose", "myapp", "docker-compose.yml")
	result := WatchLogFilePath(composeFile)
	expected := filepath.Join("/opt/compose", "myapp", "watch.log")
	if result != expected {
		t.Errorf("WatchLogFilePath(%q) = %q, want %q", composeFile, result, expected)
	}
}

func TestReadWatchPIDMissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	composeFile := filepath.Join(tmpDir, "docker-compose.yml")

	pid, err := ReadWatchPID(composeFile)
	if err != nil {
		t.Fatalf("ReadWatchPID() error = %v, want nil for missing pidfile", err)
	}
	if pid != 0 {
		t.Errorf("ReadWatchPID() = %d, want 0 for missing pidfile", pid)
	}
}

func TestReadWatchPIDValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	composeFile := filepath.Join(tmpDir, "docker-compose.yml")

	if err := os.WriteFile(WatchPidFilePath(composeFile), []byte(strconv.Itoa(12345)), 0644); err != nil {
		t.Fatalf("failed to write test pidfile: %v", err)
	}

	pid, err := ReadWatchPID(composeFile)
	if err != nil {
		t.Fatalf("ReadWatchPID() error = %v", err)
	}
	if pid != 12345 {
		t.Errorf("ReadWatchPID() = %d, want 12345", pid)
	}
}

func TestReadWatchPIDInvalidContents(t *testing.T) {
	tmpDir := t.TempDir()
	composeFile := filepath.Join(tmpDir, "docker-compose.yml")

	if err := os.WriteFile(WatchPidFilePath(composeFile), []byte("not-a-pid"), 0644); err != nil {
		t.Fatalf("failed to write test pidfile: %v", err)
	}

	if _, err := ReadWatchPID(composeFile); err == nil {
		t.Error("ReadWatchPID() error = nil, want error for non-numeric pidfile contents")
	}
}

func TestRemoveWatchPidFile(t *testing.T) {
	tmpDir := t.TempDir()
	composeFile := filepath.Join(tmpDir, "docker-compose.yml")
	pidFile := WatchPidFilePath(composeFile)

	if err := os.WriteFile(pidFile, []byte("999"), 0644); err != nil {
		t.Fatalf("failed to write test pidfile: %v", err)
	}

	if err := RemoveWatchPidFile(composeFile); err != nil {
		t.Fatalf("RemoveWatchPidFile() error = %v", err)
	}

	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Error("expected pidfile to be removed")
	}
}

func TestRemoveWatchPidFileAlreadyMissing(t *testing.T) {
	tmpDir := t.TempDir()
	composeFile := filepath.Join(tmpDir, "docker-compose.yml")

	// Should not error when the pidfile never existed.
	if err := RemoveWatchPidFile(composeFile); err != nil {
		t.Errorf("RemoveWatchPidFile() error = %v, want nil when pidfile absent", err)
	}
}

func TestIsProcessAliveCurrentProcess(t *testing.T) {
	if !IsProcessAlive(os.Getpid()) {
		t.Error("IsProcessAlive(os.Getpid()) = false, want true for the running test process")
	}
}

func TestIsProcessAliveInvalidPid(t *testing.T) {
	if IsProcessAlive(0) {
		t.Error("IsProcessAlive(0) = true, want false")
	}
	if IsProcessAlive(-1) {
		t.Error("IsProcessAlive(-1) = true, want false")
	}
}

func TestDockerClientProjectDirNested(t *testing.T) {
	client := &DockerClient{
		ProjectDirectory: "/home/user/.terraform-docker-compose",
	}

	tests := []struct {
		stack    string
		expected string
	}{
		{"simple", filepath.Join("/home/user/.terraform-docker-compose", "simple")},
		{"my-app", filepath.Join("/home/user/.terraform-docker-compose", "my-app")},
		{"prod_stack_v2", filepath.Join("/home/user/.terraform-docker-compose", "prod_stack_v2")},
	}

	for _, tt := range tests {
		t.Run(tt.stack, func(t *testing.T) {
			result := client.ProjectDir(tt.stack)
			if result != tt.expected {
				t.Errorf("ProjectDir(%q) = %q, want %q", tt.stack, result, tt.expected)
			}
		})
	}
}
