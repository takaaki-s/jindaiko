package host

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/takaaki-s/claude-code-valet/internal/config"
)

const (
	// slaveStartTimeout is the timeout for slave startup commands
	slaveStartTimeout = 30 * time.Second

	// defaultRemoteSocketPath is the default daemon socket path on the remote side
	defaultRemoteSocketPath = "~/.ccvalet/run/daemon.sock"
)

// StartSlaveCommand generates a command to start the slave daemon on a remote host
func StartSlaveCommand(hostConfig config.HostConfig) *exec.Cmd {
	socketPath := hostConfig.SocketPath
	if socketPath == "" {
		socketPath = defaultRemoteSocketPath
	}

	remoteCmd := fmt.Sprintf("ccvalet daemon start --socket %s", socketPath)

	switch hostConfig.Type {
	case "ssh":
		// Add overrides before user's ssh_opts (SSH uses first-match-wins rule)
		// - ControlMaster=no: avoid conflicts with existing ControlMaster
		// - ClearAllForwardings=yes: suppress LocalForward/RemoteForward from ssh_config
		//   (bootstrap is a one-shot command execution, no forwarding needed)
		args := make([]string, 0, len(hostConfig.SSHOpts)+6)
		args = append(args, "-o", "ControlMaster=no", "-o", "ClearAllForwardings=yes")
		args = append(args, hostConfig.SSHOpts...)
		args = append(args, hostConfig.Host, remoteCmd)
		return exec.Command("ssh", args...)
	case "docker":
		return exec.Command("docker", "exec", hostConfig.Container, "sh", "-c", remoteCmd)
	default:
		return nil
	}
}

// StartSlave starts the slave daemon on a remote host and returns the result.
// Returns an error if ccvalet is not installed on the remote host.
func StartSlave(hostConfig config.HostConfig) error {
	cmd := StartSlaveCommand(hostConfig)
	if cmd == nil {
		return fmt.Errorf("unsupported host type: %s", hostConfig.Type)
	}

	ctx, cancel := context.WithTimeout(context.Background(), slaveStartTimeout)
	defer cancel()
	cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		outStr := strings.TrimSpace(string(output))

		// Detect if ccvalet is not installed
		if isNotInstalled(outStr, err) {
			return fmt.Errorf("ccvalet is not installed on host '%s'. Install it first: go install github.com/takaaki-s/claude-code-valet/cmd/ccvalet@latest", hostConfig.ID)
		}

		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("timeout starting slave on host '%s' (waited %s)", hostConfig.ID, slaveStartTimeout)
		}

		return fmt.Errorf("failed to start slave on host '%s': %s (output: %s)", hostConfig.ID, err, outStr)
	}

	return nil
}

// isNotInstalled determines from command output whether ccvalet is not installed
func isNotInstalled(output string, err error) bool {
	lower := strings.ToLower(output)
	// Detect shell errors like "ccvalet: command not found" or "ccvalet: not found"
	// Only check lines containing "ccvalet" to distinguish from SSH infrastructure
	// errors (ControlPath etc.) that also contain "not found"
	for line := range strings.SplitSeq(lower, "\n") {
		if !strings.Contains(line, "ccvalet") {
			continue
		}
		if strings.Contains(line, "command not found") ||
			strings.Contains(line, "not found") ||
			strings.Contains(line, "no such file or directory") {
			return true
		}
	}
	// exit code 127 = command not found
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() == 127 {
			return true
		}
	}
	return false
}
