package cmd

import (
	"errors"
	"reflect"
	"testing"

	"github.com/takaaki-s/jind-ai/internal/tmux"
)

// fakeEnvWriter records SetEnvironment calls issued by pushFocusSession so
// tests can assert what tmux env writes would have occurred, without spawning
// a real tmux server. err (when non-nil) is returned from every SetEnvironment
// call to exercise the "non-tmux environment" swallow path (V-014).
type fakeEnvWriter struct {
	sets [][3]string // [session, name, value]
	err  error
}

func (f *fakeEnvWriter) SetEnvironment(session, name, value string) error {
	f.sets = append(f.sets, [3]string{session, name, value})
	return f.err
}

func TestPushFocusSession_SetsEnvOnSelection(t *testing.T) {
	fe := &fakeEnvWriter{}
	pushFocusSession("sess-abc", fe)

	want := [][3]string{{tmux.SessionName, "JIN_FOCUS_SESSION", "sess-abc"}}
	if !reflect.DeepEqual(fe.sets, want) {
		t.Errorf("sets = %v, want %v", fe.sets, want)
	}
}

// TestPushFocusSession_NoOpOnEmptySelection also stands in for daemon-
// unavailable and Esc-dismissal equivalence: the three RunE dismissal paths
// — daemon.List() failing (which returns before pushFocusSession runs at
// all), sessions=[] (Enter yields Selected() == ""), and Esc/Ctrl+C
// (Selected() == "") — all funnel into pushFocusSession("", writer) or
// bypass it entirely. Verifying the empty-string arm is no-op therefore
// covers all three from the writer's point of view without needing to
// interface-ify RunE (kept out of scope per design §11.3).
func TestPushFocusSession_NoOpOnEmptySelection(t *testing.T) {
	fe := &fakeEnvWriter{}
	pushFocusSession("", fe)

	if len(fe.sets) != 0 {
		t.Errorf("sets = %v, want none", fe.sets)
	}
}

// TestPushFocusSession_ErrorFromWriterSwallowed guards the V-014 contract:
// non-tmux invocations must not fatal. pushFocusSession discards the
// writer error; the panic-recovery guard was removed because the fake writer
// cannot panic, and go test surfaces any real regression as a normal fail.
func TestPushFocusSession_ErrorFromWriterSwallowed(t *testing.T) {
	fe := &fakeEnvWriter{err: errors.New("tmux not running")}
	pushFocusSession("sess-xyz", fe)

	if len(fe.sets) != 1 {
		t.Errorf("sets = %v, want single attempt", fe.sets)
	}
}

// TestSessionFilterPopupCmd_Registered guards the cobra wiring: init() must
// attach the hidden subcommand under rootCmd so `jin session-filter-popup`
// resolves. This is the minimum shape check that catches "forgot to
// AddCommand" regressions without needing to run the bubbletea program.
func TestSessionFilterPopupCmd_Registered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"session-filter-popup"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd == nil || cmd.Name() != "session-filter-popup" {
		t.Fatalf("session-filter-popup not registered: %+v", cmd)
	}
}

// TestSessionFilterPopupCmd_Hidden regresses the "internal use only" flag.
// The command must remain absent from `jin --help` so users don't invoke it
// directly; the tmux popup shells out to it.
func TestSessionFilterPopupCmd_Hidden(t *testing.T) {
	if !sessionFilterPopupCmd.Hidden {
		t.Errorf("session-filter-popup should be Hidden")
	}
}

// TestSessionFilterPopupCmd_RunE confirms RunE is wired. Without it, cobra
// would silently print help text on invocation instead of running the
// picker. Testing the closure body itself needs interface-ification of
// daemon.Client / tea.Program (design §11.3 says out of scope for this PR).
func TestSessionFilterPopupCmd_RunE(t *testing.T) {
	if sessionFilterPopupCmd.RunE == nil {
		t.Errorf("session-filter-popup.RunE = nil, want a runner")
	}
}
