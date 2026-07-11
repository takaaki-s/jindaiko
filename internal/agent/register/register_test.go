package register_test

import (
	"testing"

	"github.com/takaaki-s/jind-ai/internal/agent"
	// Blank import triggers init() in the register package, which is the
	// side-effect under test. Without this, agent.Registry stays empty in
	// tests because the daemon-side blank import from cmd/jin doesn't
	// reach here.
	_ "github.com/takaaki-s/jind-ai/internal/agent/register"
)

// TestRegisterInit_RegistersKnownKinds is the guardrail for `jin session
// new --agent codex`: if someone deletes the codex import or Register
// call in register.go, the daemon would silently reject Codex sessions
// with an "unknown agent kind" error at spawn time. Failing this test
// forces the mistake to be caught in CI.
func TestRegisterInit_RegistersKnownKinds(t *testing.T) {
	want := map[string]bool{
		"claude": false,
		"codex":  false,
	}
	for _, k := range agent.Kinds() {
		if _, ok := want[k]; ok {
			want[k] = true
		}
	}
	for k, seen := range want {
		if !seen {
			t.Errorf("registry missing %q — check register.go", k)
		}
	}
}

func TestRegisterInit_LookupCodex(t *testing.T) {
	// Beyond presence in Kinds(): the actual Codex adapter object must
	// come back from Lookup and identify itself correctly.
	a, err := agent.Lookup("codex")
	if err != nil {
		t.Fatalf("Lookup(codex): %v", err)
	}
	if a.Kind() != "codex" {
		t.Errorf("Kind() = %q, want %q", a.Kind(), "codex")
	}
}
