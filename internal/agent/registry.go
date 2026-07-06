package agent

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// The registry is process-global mutable state: every adapter registers at
// program start (via a blank import from cmd/jin/cmd/root.go) and the daemon
// looks agents up when spawning or interpreting hook events.
//
// The mutex protects concurrent Register + Lookup calls; in practice registration
// only happens during init() but the lock keeps behaviour well-defined if a
// caller ever registers late (e.g. from tests).
var (
	regMu sync.RWMutex
	reg   = map[string]Agent{}
)

// Register adds an agent adapter to the process-global registry.
//
// Registering the same kind twice panics — this is a programmer error that
// should surface at start-up, not at first Lookup.
func Register(a Agent) {
	if a == nil {
		panic("agent: Register called with nil Agent")
	}
	kind := a.Kind()
	if kind == "" {
		panic("agent: Register called with Agent whose Kind() is empty")
	}
	regMu.Lock()
	defer regMu.Unlock()
	if _, dup := reg[kind]; dup {
		panic(fmt.Sprintf("agent: duplicate kind %q", kind))
	}
	reg[kind] = a
}

// Lookup returns the adapter registered for kind, or an error whose message
// lists every currently-known kind. The error text is user-facing (surfaced
// by "jin session new --agent <kind>"), so keep it terse.
func Lookup(kind string) (Agent, error) {
	regMu.RLock()
	defer regMu.RUnlock()
	a, ok := reg[kind]
	if !ok {
		return nil, fmt.Errorf("unknown agent kind: %s. available: %s", kind, availableListLocked())
	}
	return a, nil
}

// Kinds returns the sorted list of registered kinds. Useful for CLI/TUI
// selectors and for constructing the "available" list in error messages.
func Kinds() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(reg))
	for k := range reg {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// availableListLocked returns "a, b, c" from the registry. Caller must hold
// regMu (RLock is fine); factored out only to keep Lookup's error path readable.
func availableListLocked() string {
	names := make([]string, 0, len(reg))
	for k := range reg {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// ResetRegistryForTest wipes every registered adapter. The verbose name is
// intentional: this must never be called from production code (registry
// state is process-global and Register + Lookup assume kinds don't vanish
// mid-session). internal/agent/agenttest.Reset wraps this for test call
// sites that want a shorter name.
func ResetRegistryForTest() {
	regMu.Lock()
	defer regMu.Unlock()
	reg = map[string]Agent{}
}
