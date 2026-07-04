package tunnel

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/takaaki-s/honjin/internal/config"
	"github.com/takaaki-s/honjin/internal/paths"
)

// localSocketDir is the directory for tunnel local sockets.
const localSocketDir = "/tmp/jin-tunnels"

// Tunnel represents a tunnel connection to a remote host
type Tunnel struct {
	HostID      string
	HostType    string // "ssh" or "docker"
	LocalSocket string // Local socket path
	process     *os.Process
	cmd         *exec.Cmd
}

// TunnelOptions configures optional tunnel behavior
type TunnelOptions struct {
	ReverseEnabled    bool   // Enable reverse tunnel (-R) for bidirectional routing
	LocalHostID       string // Master daemon's host ID (used for remote peer socket path)
	LocalDaemonSocket string // Master daemon's local socket path
}

// PeerSocketDir is the directory for reverse tunnel peer sockets on the remote side.
// Located in /tmp which is assumed to be single-user (e.g., EC2 instances).
const PeerSocketDir = "/tmp/jin-peers"

// Manager manages tunnel connections
type Manager struct {
	mu      sync.RWMutex
	tunnels map[string]*Tunnel
}

// NewManager creates a new tunnel manager
func NewManager() *Manager {
	return &Manager{
		tunnels: make(map[string]*Tunnel),
	}
}

// Open opens a tunnel based on host configuration
func (m *Manager) Open(hostConfig config.HostConfig, opts ...TunnelOptions) (string, error) {
	switch hostConfig.Type {
	case "ssh":
		return m.OpenSSH(hostConfig, opts...)
	case "docker":
		return m.OpenDocker(hostConfig)
	default:
		return "", fmt.Errorf("unsupported host type: %s", hostConfig.Type)
	}
}

// OpenSSH opens an SSH tunnel to forward the remote daemon socket
// ssh -L {localSocket}:{remoteSocket} -N -f {host}
func (m *Manager) OpenSSH(hostConfig config.HostConfig, opts ...TunnelOptions) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return existing socket if already connected
	if t, ok := m.tunnels[hostConfig.ID]; ok {
		if m.isAlive(t) {
			return t.LocalSocket, nil
		}
		// Clean up and reconnect if dead
		_ = m.closeLocked(hostConfig.ID)
	}

	// Generate local socket path
	localSocket, err := m.prepareLocalSocket(hostConfig.ID)
	if err != nil {
		return "", fmt.Errorf("failed to prepare local socket: %w", err)
	}

	// Determine remote socket path
	// SSH -L requires an absolute path for the remote side (relative paths are not resolved correctly by sshd)
	remoteSocket := hostConfig.SocketPath
	if remoteSocket == "" {
		// Get remote home directory to build an absolute path
		remoteHome, err := m.getRemoteHome(hostConfig)
		if err != nil {
			os.Remove(localSocket)
			return "", fmt.Errorf("failed to get remote home directory: %w", err)
		}
		remoteSocket = remoteHome + "/" + paths.RemoteDefaultSocketRel()
	}

	// Build SSH command
	// Add overrides before user's ssh_opts (SSH uses first-match-wins rule)
	// - ControlMaster=no: don't use ControlMaster for long-lived tunnel connections
	// - ExitOnForwardFailure=no: don't abort SSH if LocalForward/RemoteForward in
	//   ssh_config fails due to port conflicts (tunnel -L is verified separately via waitForSocket)
	// - -A: enable SSH agent forwarding (needed for git fetch etc. on remote)
	// Note: ClearAllForwardings=yes cannot be used as it also clears command-line -L
	args := make([]string, 0, len(hostConfig.SSHOpts)+10)
	args = append(args, "-A",
		"-o", "ControlMaster=no",
		"-o", "ExitOnForwardFailure=no",
		"-o", "ServerAliveInterval=15",
		"-o", "ServerAliveCountMax=3",
	)
	args = append(args, hostConfig.SSHOpts...)
	// Create a stable symlink for the SSH agent socket on the remote,
	// so the slave daemon can use it for git fetch etc.
	// Instead of -N (no command), create the symlink then sleep to keep the connection.
	// Add reverse tunnel (-R) for bidirectional routing if requested
	var reversePreamble string
	if len(opts) > 0 && opts[0].ReverseEnabled && opts[0].LocalHostID != "" {
		reverseSockRemote := fmt.Sprintf("%s/%s/daemon.sock", PeerSocketDir, opts[0].LocalHostID)
		reverseSockLocal := opts[0].LocalDaemonSocket
		args = append(args, "-R", reverseSockRemote+":"+reverseSockLocal)
		// Ensure peer directory exists and clean up stale socket on remote
		reversePreamble = fmt.Sprintf(
			"mkdir -p %s/%s && rm -f %s; ",
			PeerSocketDir, opts[0].LocalHostID, reverseSockRemote,
		)
	}

	remoteStateDir := "~/" + paths.RemoteStateDirRel()
	agentSymlink := remoteStateDir + "/ssh-agent.sock"
	remoteCmd := fmt.Sprintf(
		"%smkdir -p %s && test -n \"$SSH_AUTH_SOCK\" && ln -sf \"$SSH_AUTH_SOCK\" %s; "+
			"while sleep 3600; do :; done",
		reversePreamble, remoteStateDir, agentSymlink,
	)
	args = append(args,
		"-L", localSocket+":"+remoteSocket,
		hostConfig.Host,
		remoteCmd,
	)

	cmd := exec.Command("ssh", args...)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		os.Remove(localSocket)
		return "", fmt.Errorf("failed to start SSH tunnel to %s: %w", hostConfig.Host, err)
	}

	tunnel := &Tunnel{
		HostID:      hostConfig.ID,
		HostType:    "ssh",
		LocalSocket: localSocket,
		process:     cmd.Process,
		cmd:         cmd,
	}
	m.tunnels[hostConfig.ID] = tunnel

	// Wait until the socket becomes available
	if err := m.waitForSocket(localSocket, 10*time.Second); err != nil {
		_ = m.closeLocked(hostConfig.ID)
		return "", fmt.Errorf("SSH tunnel to %s started but socket not available: %w", hostConfig.Host, err)
	}

	return localSocket, nil
}

// OpenDocker sets up a connection to the Docker container's daemon socket.
// For Docker, the socket is expected to be shared via volume mount.
func (m *Manager) OpenDocker(hostConfig config.HostConfig) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return existing socket if already configured
	if t, ok := m.tunnels[hostConfig.ID]; ok {
		return t.LocalSocket, nil
	}

	// For Docker, the socket path is assumed to be directly accessible via volume mount.
	// The local socket path is auto-calculated from host ID using the same convention as SSH.
	// Example: docker run -v /tmp/jin-tunnels/docker-dev:/root/.local/state/honjin container
	//          -> accessible at /tmp/jin-tunnels/docker-dev/daemon.sock
	localSocket := filepath.Join(localSocketDir, hostConfig.ID, "daemon.sock")
	_ = os.MkdirAll(filepath.Dir(localSocket), 0700)

	tunnel := &Tunnel{
		HostID:      hostConfig.ID,
		HostType:    "docker",
		LocalSocket: localSocket,
		process:     nil, // No process management needed for Docker
		cmd:         nil,
	}
	m.tunnels[hostConfig.ID] = tunnel

	return localSocket, nil
}

// Close closes the tunnel for the specified host
func (m *Manager) Close(hostID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closeLocked(hostID)
}

// CloseAll closes all tunnels
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for hostID := range m.tunnels {
		_ = m.closeLocked(hostID)
	}
}

// LocalSocket returns the local socket path for the given host ID
func (m *Manager) LocalSocket(hostID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if t, ok := m.tunnels[hostID]; ok {
		return t.LocalSocket
	}
	return ""
}

// IsAlive returns whether the tunnel is alive
func (m *Manager) IsAlive(hostID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.tunnels[hostID]
	if !ok {
		return false
	}
	return m.isAlive(t)
}

// closeLocked closes the tunnel (caller must hold the lock)
func (m *Manager) closeLocked(hostID string) error {
	t, ok := m.tunnels[hostID]
	if !ok {
		return nil
	}

	// Terminate SSH process
	if t.process != nil {
		_ = t.process.Kill()
		if t.cmd != nil {
			_ = t.cmd.Wait()
		}
	}

	// Remove local socket file
	if t.HostType == "ssh" {
		os.Remove(t.LocalSocket)
	}

	delete(m.tunnels, hostID)
	return nil
}

// isAlive checks whether the tunnel is alive
func (m *Manager) isAlive(t *Tunnel) bool {
	if t.HostType == "docker" {
		// For Docker, check socket file existence
		_, err := os.Stat(t.LocalSocket)
		return err == nil
	}

	// For SSH, check process liveness
	if t.process == nil {
		return false
	}
	// On Unix, send signal 0 to check process liveness without disturbing it.
	// syscall.Signal(0) must be used; os.Signal(nil) is a nil interface and
	// always returns "unsupported signal type", making IsAlive always false.
	err := t.process.Signal(syscall.Signal(0))
	return err == nil
}

// prepareLocalSocket prepares the local socket path
func (m *Manager) prepareLocalSocket(hostID string) (string, error) {
	dir := filepath.Join(localSocketDir, hostID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}

	socketPath := filepath.Join(dir, "daemon.sock")

	// Remove existing socket file if present
	os.Remove(socketPath)

	return socketPath, nil
}

// waitForSocket waits until the socket becomes available
func (m *Manager) waitForSocket(socketPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		conn, err := net.Dial("unix", socketPath)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for socket %s", socketPath)
}

// getRemoteHome gets the remote home directory via SSH
func (m *Manager) getRemoteHome(hostConfig config.HostConfig) (string, error) {
	args := []string{"-o", "ControlMaster=no", "-o", "ClearAllForwardings=yes"}
	args = append(args, hostConfig.SSHOpts...)
	args = append(args, hostConfig.Host, "echo $HOME")

	cmd := exec.Command("ssh", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	home := strings.TrimSpace(string(out))
	if home == "" {
		return "", fmt.Errorf("empty home directory")
	}
	return home, nil
}
