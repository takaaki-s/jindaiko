package tunnel

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takaaki-s/honjin/internal/config"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.tunnels == nil {
		t.Fatal("tunnels map is nil, expected initialized map")
	}
	if len(m.tunnels) != 0 {
		t.Fatalf("tunnels map should be empty, got %d entries", len(m.tunnels))
	}
}

func TestManager_LocalSocket_NotFound(t *testing.T) {
	m := NewManager()
	got := m.LocalSocket("nonexistent-host")
	if got != "" {
		t.Fatalf("LocalSocket for unknown hostID: got %q, want empty string", got)
	}
}

func TestManager_LocalSocket_Found(t *testing.T) {
	m := NewManager()
	expectedPath := "/tmp/jin-tunnels/test-host/daemon.sock"
	m.tunnels["test-host"] = &Tunnel{
		HostID:      "test-host",
		HostType:    "ssh",
		LocalSocket: expectedPath,
	}

	got := m.LocalSocket("test-host")
	if got != expectedPath {
		t.Fatalf("LocalSocket: got %q, want %q", got, expectedPath)
	}
}

func TestManager_IsAlive_NotFound(t *testing.T) {
	m := NewManager()
	if m.IsAlive("nonexistent-host") {
		t.Fatal("IsAlive for unknown hostID should return false")
	}
}

// TestManager_IsAlive_SSHProcess verifies that isAlive correctly returns true
// for a running process and false after it exits.
// This test guards against the os.Signal(nil) bug, where passing a nil os.Signal
// interface always returned "unsupported signal type", making IsAlive always false
// and causing spurious reconnects every watchRemoteConnections interval.
func TestManager_IsAlive_SSHProcess(t *testing.T) {
	m := NewManager()

	cmd := exec.Command("sleep", "3600")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start process: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	tunnel := &Tunnel{
		HostID:   "test-ssh",
		HostType: "ssh",
		process:  cmd.Process,
		cmd:      cmd,
	}
	m.tunnels["test-ssh"] = tunnel

	if !m.isAlive(tunnel) {
		t.Error("isAlive should return true for a running process")
	}
	if !m.IsAlive("test-ssh") {
		t.Error("IsAlive should return true for a running process")
	}

	_ = cmd.Process.Kill()
	_ = cmd.Wait()

	if m.isAlive(tunnel) {
		t.Error("isAlive should return false after process exits")
	}
}

func TestManager_CloseAll_Empty(t *testing.T) {
	m := NewManager()
	// Should not panic
	m.CloseAll()
	if len(m.tunnels) != 0 {
		t.Fatalf("tunnels map should still be empty after CloseAll, got %d entries", len(m.tunnels))
	}
}

func TestManager_Close_NotFound(t *testing.T) {
	m := NewManager()
	err := m.Close("nonexistent-host")
	if err != nil {
		t.Fatalf("Close for unknown hostID should return nil, got %v", err)
	}
}

func TestManager_PrepareLocalSocket(t *testing.T) {
	m := NewManager()

	// Use a unique hostID based on test temp dir basename to avoid collisions
	tmpDir := t.TempDir()
	hostID := "test-prepare-" + filepath.Base(tmpDir)

	socketPath, err := m.prepareLocalSocket(hostID)
	if err != nil {
		t.Fatalf("prepareLocalSocket returned error: %v", err)
	}

	// Verify the socket path has the expected structure
	expectedDir := filepath.Join(localSocketDir, hostID)
	expectedSocket := filepath.Join(expectedDir, "daemon.sock")
	if socketPath != expectedSocket {
		t.Fatalf("prepareLocalSocket: got %q, want %q", socketPath, expectedSocket)
	}

	// Verify the directory was created
	info, err := os.Stat(expectedDir)
	if err != nil {
		t.Fatalf("directory %q was not created: %v", expectedDir, err)
	}
	if !info.IsDir() {
		t.Fatalf("%q is not a directory", expectedDir)
	}

	// Cleanup: remove the created directory
	t.Cleanup(func() {
		os.RemoveAll(filepath.Join(localSocketDir, hostID))
	})
}

func TestManager_Open_UnsupportedType(t *testing.T) {
	m := NewManager()
	hostConfig := config.HostConfig{
		ID:   "test-host",
		Type: "unknown",
	}

	_, err := m.Open(hostConfig)
	if err == nil {
		t.Fatal("Open with unsupported type should return error")
	}
	if !strings.Contains(err.Error(), "unsupported host type") {
		t.Fatalf("error message should contain 'unsupported host type', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("error message should contain the type 'unknown', got %q", err.Error())
	}
}

func TestManager_IsAlive_DockerSocket(t *testing.T) {
	m := NewManager()
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "daemon.sock")

	// Insert a docker tunnel with a socket path in our temp dir
	m.tunnels["docker-test"] = &Tunnel{
		HostID:      "docker-test",
		HostType:    "docker",
		LocalSocket: socketPath,
	}

	// Socket file does not exist yet, should not be alive
	if m.IsAlive("docker-test") {
		t.Fatal("IsAlive should return false when docker socket file does not exist")
	}

	// Create the socket file
	f, err := os.Create(socketPath)
	if err != nil {
		t.Fatalf("failed to create socket file: %v", err)
	}
	f.Close()

	// Now it should be alive
	if !m.IsAlive("docker-test") {
		t.Fatal("IsAlive should return true when docker socket file exists")
	}

	// Remove the socket file
	os.Remove(socketPath)

	// Should be not alive again
	if m.IsAlive("docker-test") {
		t.Fatal("IsAlive should return false after docker socket file is removed")
	}
}
