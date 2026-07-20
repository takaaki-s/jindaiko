package opencode

import (
	"github.com/takaaki-s/jind-ai/internal/agent"
	"github.com/takaaki-s/jind-ai/internal/session"
)

// Canonical event names. opencode's own bus vocabulary (session.created,
// session.status, session.idle, permission.asked, ...) does not line up with
// the names HandleHookEvent keys its agent-agnostic side effects on, so the
// bundled plugin translates before calling `jin hook`. Keeping the constants
// here — next to the switch that consumes them — makes the contract with
// plugin/jin.ts explicit; the mapping table lives in that file's header.
const (
	eventSessionStart     = "SessionStart"
	eventUserPromptSubmit = "UserPromptSubmit"
	eventPermission       = "PermissionRequest"
	eventStop             = "Stop"
	eventStopFailure      = "StopFailure"
)

// errorMessage is what StopFailure surfaces on the session. opencode's
// session.error payload carries a structured error whose shape varies by
// provider, and the plugin deliberately does not try to flatten it into a
// message, so the adapter reports a fixed string and leaves the detail to
// the agent's own pane output.
const errorMessage = "opencode reported an error"

// EventStatusSource translates the canonical event names the bundled plugin
// emits into StatusUpdates.
//
// Stateless, but held as a pointer so a future revision can add per-session
// memoisation without changing callers — same shape as the Codex adapter's
// HookStatusSource.
type EventStatusSource struct{}

// NewEventStatusSource constructs the interpreter.
func NewEventStatusSource() *EventStatusSource { return &EventStatusSource{} }

// Interpret implements session.StatusSource.
//
//	SessionStart       (zero, false)          side effects only (AgentSessionStarted, id re-key)
//	UserPromptSubmit   thinking + ClearError  turn started, or permission just answered
//	PermissionRequest  permission             agent is blocked on the user
//	Stop               idle + ClearError      turn finished
//	StopFailure        idle + ErrorMessage    turn ended in an error
//
// A false verdict means "meaningful, but no status change" — Manager still
// runs its agent-agnostic bookkeeping for SessionStart, which is exactly how
// Session.AgentSessionID gets re-keyed from the pre-minted UUID to
// opencode's real ses_… id.
func (s *EventStatusSource) Interpret(sig agent.StatusSignal) (agent.StatusUpdate, bool) {
	if sig.Kind != "hook" {
		return agent.StatusUpdate{}, false
	}
	switch sig.Payload["event"] {
	case eventUserPromptSubmit:
		return agent.StatusUpdate{
			Status:     session.StatusThinking,
			ClearError: true,
			Notify:     agent.NotifyNone,
		}, true
	case eventPermission:
		return agent.StatusUpdate{
			Status: session.StatusPermission,
			Notify: agent.NotifyPermission,
		}, true
	case eventStop:
		return agent.StatusUpdate{
			Status:     session.StatusIdle,
			ClearError: true,
			Notify:     agent.NotifyTaskComplete,
		}, true
	case eventStopFailure:
		return agent.StatusUpdate{
			Status:       session.StatusIdle,
			ErrorMessage: errorMessage,
			Notify:       agent.NotifyError,
		}, true
	}
	// SessionStart and anything we don't recognise (opencode emits a large
	// bus vocabulary the plugin filters, but new event names can always
	// appear) leave Status untouched.
	return agent.StatusUpdate{}, false
}
