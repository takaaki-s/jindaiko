package cmd

import (
	"github.com/takaaki-s/honjin/internal/paths"
)

// getConfigDir returns the user configuration directory (XDG-compliant).
func getConfigDir() string {
	return paths.Config()
}

// getStateDir returns the persistent state directory (XDG-compliant).
func getStateDir() string {
	return paths.State()
}
