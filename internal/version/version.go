package version

import "fmt"

// These variables are set at build time via -ldflags.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Full returns a formatted version string.
func Full() string {
	return fmt.Sprintf("%s (commit: %s, built: %s)", Version, Commit, Date)
}
