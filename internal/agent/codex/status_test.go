package codex

import (
	"testing"

	"github.com/takaaki-s/jind-ai/internal/agent"
	"github.com/takaaki-s/jind-ai/internal/session"
)

func TestHookStatusSource_EventMapping(t *testing.T) {
	tests := []struct {
		event      string
		wantStatus session.Status
		wantClear  bool
		wantNotify agent.NotifyKind
		wantOK     bool
	}{
		{"UserPromptSubmit", session.StatusThinking, true, agent.NotifyNone, true},
		{"PreToolUse", session.StatusThinking, true, agent.NotifyNone, true},
		{"PostToolUse", session.StatusThinking, true, agent.NotifyNone, true},
		{"PermissionRequest", session.StatusPermission, false, agent.NotifyPermission, true},
		{"Stop", session.StatusIdle, true, agent.NotifyTaskComplete, true},
		// SessionStart: no status change, Manager owns the side effects.
		{"SessionStart", "", false, agent.NotifyNone, false},
		// Unknown event: parser must fall through cleanly.
		{"CompletelyUnknownEvent", "", false, agent.NotifyNone, false},
	}

	src := NewHookStatusSource()
	for _, tc := range tests {
		t.Run(tc.event, func(t *testing.T) {
			sig := agent.StatusSignal{
				Kind:    "hook",
				Payload: map[string]string{"event": tc.event},
			}
			got, ok := src.Interpret(sig)
			if ok != tc.wantOK {
				t.Errorf("ok = %v, want %v", ok, tc.wantOK)
			}
			if got.Status != tc.wantStatus {
				t.Errorf("Status = %q, want %q", got.Status, tc.wantStatus)
			}
			if got.ClearError != tc.wantClear {
				t.Errorf("ClearError = %v, want %v", got.ClearError, tc.wantClear)
			}
			if got.Notify != tc.wantNotify {
				t.Errorf("Notify = %q, want %q", got.Notify, tc.wantNotify)
			}
		})
	}
}

func TestHookStatusSource_NonHookKind(t *testing.T) {
	// Any Kind other than "hook" — pane-tail, poll, whatever — must be
	// ignored. Manager may route non-hook signals through here in the
	// future, and returning ok=true for them would trip a status write on
	// a signal the adapter did not actually understand.
	src := NewHookStatusSource()
	for _, k := range []string{"", "pane-tail", "poll", "running"} {
		sig := agent.StatusSignal{
			Kind:    k,
			Payload: map[string]string{"event": "UserPromptSubmit"},
		}
		if got, ok := src.Interpret(sig); ok {
			t.Errorf("Kind=%q returned ok=true with %+v; want ignored", k, got)
		}
	}
}

func TestHookStatusSource_MissingEventField(t *testing.T) {
	// A hook signal without an "event" key is malformed; the adapter must
	// treat it as unknown rather than pick up whatever the zero-value
	// switch case would fall into.
	src := NewHookStatusSource()
	sig := agent.StatusSignal{
		Kind:    "hook",
		Payload: map[string]string{},
	}
	if got, ok := src.Interpret(sig); ok {
		t.Errorf("missing event returned ok=true with %+v", got)
	}
}

func TestHookStatusSource_ImplementsInterface(t *testing.T) {
	// Compile-time interface check: HookStatusSource must satisfy the
	// session.StatusSource contract Manager holds it through.
	var _ session.StatusSource = (*HookStatusSource)(nil)
	var _ session.StatusSource = NewHookStatusSource()
}
