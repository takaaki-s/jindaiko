package cmd

import (
	"os"

	"github.com/takaaki-s/jind-ai/internal/paths"
	"github.com/takaaki-s/jind-ai/internal/tmux"
)

// getConfigDir returns the user configuration directory (XDG-compliant).
func getConfigDir() string {
	return paths.Config()
}

// getStateDir returns the persistent state directory (XDG-compliant).
func getStateDir() string {
	return paths.State()
}

// ensureSSHAuthSockFromTmux copies SSH_AUTH_SOCK from the outer tmux server
// environment into this process when the environment inherited from the
// parent shell did not carry one. tmux popup children can start under a
// stripped environment; this restores agent access for commands that rely
// on it. No-op when SSH_AUTH_SOCK is already set or when tmux has no value.
func ensureSSHAuthSockFromTmux(tc *tmux.Client) {
	if os.Getenv("SSH_AUTH_SOCK") != "" {
		return
	}
	if sock := tc.GetEnvironment(tmux.SessionName, "SSH_AUTH_SOCK"); sock != "" {
		_ = os.Setenv("SSH_AUTH_SOCK", sock)
	}
}
