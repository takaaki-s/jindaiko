package host

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode"

	"github.com/takaaki-s/honjin/internal/config"
	"github.com/takaaki-s/honjin/internal/paths"
)

// slaveStartTimeout is the timeout for slave startup commands.
const slaveStartTimeout = 30 * time.Second

// BootstrapOptions configures optional peer information for the slave daemon
type BootstrapOptions struct {
	PeerSocketPath string // Reverse tunnel socket path on remote (e.g., /tmp/jin-peers/mac/daemon.sock)
	PeerHostID     string // Master daemon's host ID
}

// ValidateIdentifier checks that a string contains only safe characters for use
// in shell commands (letters, digits, hyphens, underscores).
func ValidateIdentifier(s string) error {
	if s == "" {
		return fmt.Errorf("identifier must not be empty")
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return fmt.Errorf("invalid character %q in identifier %q", r, s)
		}
	}
	return nil
}

// validatePath checks that a string contains only safe characters for a file path.
func validatePath(s string) error {
	if s == "" {
		return fmt.Errorf("path must not be empty")
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) &&
			r != '-' && r != '_' && r != '/' && r != '.' && r != '~' {
			return fmt.Errorf("invalid character %q in path %q", r, s)
		}
	}
	return nil
}

// StartSlaveCommand generates a command to start the slave daemon on a remote host
func StartSlaveCommand(hostConfig config.HostConfig, opts ...BootstrapOptions) *exec.Cmd {
	socketPath := hostConfig.SocketPath
	if socketPath == "" {
		socketPath = paths.RemoteDefaultSocket()
	}

	jinBin := "jin"
	if hostConfig.JinPath != "" {
		jinBin = hostConfig.JinPath
	}

	remoteCmd := fmt.Sprintf("%s daemon start --socket %s", jinBin, socketPath)
	if len(opts) > 0 && opts[0].PeerSocketPath != "" && opts[0].PeerHostID != "" {
		remoteCmd += fmt.Sprintf(" --peer-socket %s --peer-id %s", opts[0].PeerSocketPath, opts[0].PeerHostID)
	}

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
// Returns an error if jin is not installed on the remote host.
func StartSlave(hostConfig config.HostConfig, opts ...BootstrapOptions) error {
	// Validate all inputs before building the shell command (prevent injection)
	if hostConfig.JinPath != "" {
		if err := validatePath(hostConfig.JinPath); err != nil {
			return fmt.Errorf("invalid jin_path: %w", err)
		}
	}
	if hostConfig.SocketPath != "" {
		if err := validatePath(hostConfig.SocketPath); err != nil {
			return fmt.Errorf("invalid socket path: %w", err)
		}
	}
	if len(opts) > 0 && opts[0].PeerSocketPath != "" && opts[0].PeerHostID != "" {
		if err := validatePath(opts[0].PeerSocketPath); err != nil {
			return fmt.Errorf("invalid peer socket path: %w", err)
		}
		if err := ValidateIdentifier(opts[0].PeerHostID); err != nil {
			return fmt.Errorf("invalid peer host ID: %w", err)
		}
	}

	cmd := StartSlaveCommand(hostConfig, opts...)
	if cmd == nil {
		return fmt.Errorf("unsupported host type: %s", hostConfig.Type)
	}

	ctx, cancel := context.WithTimeout(context.Background(), slaveStartTimeout)
	defer cancel()
	cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		outStr := strings.TrimSpace(string(output))

		// Detect if jin is not installed
		if isNotInstalled(outStr, err) {
			return fmt.Errorf("jin is not installed on host '%s'. Install it first: go install github.com/takaaki-s/honjin/cmd/jin@latest", hostConfig.ID)
		}

		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("timeout starting slave on host '%s' (waited %s)", hostConfig.ID, slaveStartTimeout)
		}

		return fmt.Errorf("failed to start slave on host '%s': %s (output: %s)", hostConfig.ID, err, outStr)
	}

	return nil
}

// isNotInstalled determines from command output whether jin is not installed
func isNotInstalled(output string, err error) bool {
	lower := strings.ToLower(output)
	// Detect shell errors like "jin: command not found" or "jin: not found"
	// Only check lines containing "jin" to distinguish from SSH infrastructure
	// errors (ControlPath etc.) that also contain "not found"
	for line := range strings.SplitSeq(lower, "\n") {
		if !strings.Contains(line, "jin") {
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
