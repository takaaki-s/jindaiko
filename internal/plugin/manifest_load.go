// Package plugin owns the runtime side of jind-ai plugins — installation,
// registry classification, event dispatch, and per-run execution. The
// manifest itself lives in pkg/plugin/manifest (the single source of truth
// shared with the registry crawler); this file exposes a load helper that
// wraps parse + Validate so callers get an error-shaped API, and holds the
// current jin binary version used for compat checks.
package plugin

import (
	"fmt"
	"sync/atomic"

	"github.com/takaaki-s/jind-ai/internal/version"
	"github.com/takaaki-s/jind-ai/pkg/plugin/manifest"
)

// jinVersionPtr is the current jin binary version used by every plugin
// compat check in this package. It is seeded from the ldflags-driven
// internal/version.Version at package init and can be swapped by
// SetJinVersion. An atomic.Pointer keeps concurrent reads (each plugin
// classify runs in the dispatcher's goroutines) race-free without a
// mutex, and prevents test overrides from tripping -race against a
// still-running publish goroutine.
var jinVersionPtr atomic.Pointer[string]

func init() {
	v := version.Version
	jinVersionPtr.Store(&v)
}

// SetJinVersion overrides the jin version used for compat checks. Intended
// for tests and for host binaries that resolve the version through a
// source other than internal/version at process start. Returns the
// previous value so callers can restore it.
func SetJinVersion(v string) string {
	prevPtr := jinVersionPtr.Load()
	next := v
	jinVersionPtr.Store(&next)
	if prevPtr == nil {
		return ""
	}
	return *prevPtr
}

// currentJinVersion returns the version last written by SetJinVersion (or
// the init snapshot of internal/version.Version). Reads are lock-free.
func currentJinVersion() string {
	if p := jinVersionPtr.Load(); p != nil {
		return *p
	}
	return ""
}

// CurrentJinVersion returns the jin binary version used by plugin compat
// checks (i.e. what checkJinCompat matches against). Exported for the CLI's
// consent screen — the same value the check itself uses.
func CurrentJinVersion() string { return currentJinVersion() }

// loadManifest reads and validates the manifest at pluginDir. Unknown fields
// are dropped here on purpose — they are advisory WARNs for the validate
// command, not blockers for the runtime. Every ERROR-severity rule turns
// into a wrapped error so callers can bubble it up unchanged.
func loadManifest(pluginDir string) (*manifest.Manifest, error) {
	m, _, err := manifest.LoadFile(pluginDir)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	if err := manifest.Validate(m); err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}
	return m, nil
}

// checkJinCompat reports whether the running jin binary satisfies m's jin
// compat range. Development builds (Version == "dev" or unset) are treated
// as satisfying every range so local plugin development is unblocked.
func checkJinCompat(m *manifest.Manifest) error {
	return manifest.CheckJinCompat(m.Jin, currentJinVersion())
}
