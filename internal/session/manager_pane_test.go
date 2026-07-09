package session

import (
	"testing"
)

// paneTestSession creates a stopped session and sets its tmux fields under the
// manager lock, mirroring how running sessions are simulated elsewhere in this
// package (see TestManager_Kill).
func paneTestSession(t *testing.T, mgr *Manager, workDir, paneID, windowName string) *Session {
	t.Helper()
	sess, _, err := mgr.CreateWithOptions(CreateOptions{WorkDir: workDir, Description: "pane-test"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	mgr.mu.Lock()
	sess.TmuxPaneID = paneID
	sess.TmuxWindowName = windowName
	mgr.mu.Unlock()
	return sess
}

func TestManager_PaneTarget_PaneID(t *testing.T) {
	mgr, _, _ := newTestManager(t)
	sess := paneTestSession(t, mgr, "/tmp/pane-a", "%42", "sess-x")

	got, err := mgr.PaneTarget(sess.ID)
	if err != nil {
		t.Fatalf("PaneTarget failed: %v", err)
	}
	if got != "%42" {
		t.Errorf("PaneTarget = %q, want %q", got, "%42")
	}
}

func TestManager_PaneTarget_WindowFallback(t *testing.T) {
	mgr, _, _ := newTestManager(t)
	sess := paneTestSession(t, mgr, "/tmp/pane-b", "", "sess-y")

	got, err := mgr.PaneTarget(sess.ID)
	if err != nil {
		t.Fatalf("PaneTarget failed: %v", err)
	}
	want := "jin:sess-y.0"
	if got != want {
		t.Errorf("PaneTarget = %q, want %q", got, want)
	}
}

func TestManager_PaneTarget_NoPane(t *testing.T) {
	mgr, _, _ := newTestManager(t)
	sess := paneTestSession(t, mgr, "/tmp/pane-c", "", "")

	if _, err := mgr.PaneTarget(sess.ID); err == nil {
		t.Fatal("expected error when session has no pane, got nil")
	}
}

func TestManager_PaneTarget_UnknownID(t *testing.T) {
	mgr, _, _ := newTestManager(t)

	if _, err := mgr.PaneTarget("does-not-exist"); err == nil {
		t.Fatal("expected error for unknown id, got nil")
	}
}

func TestManager_PanePopup_TargetAndDir(t *testing.T) {
	mgr, mock, _ := newTestManager(t)
	sess := paneTestSession(t, mgr, "/tmp/pane-popup", "%7", "sess-z")

	if err := mgr.PanePopup(sess.ID, "echo hi", "Title", "80%", "50%"); err != nil {
		t.Fatalf("PanePopup failed: %v", err)
	}

	var found bool
	for _, c := range mock.calls {
		if c.method != "DisplayPopup" {
			continue
		}
		found = true
		if c.args[0] != "%7" {
			t.Errorf("DisplayPopup target = %q, want %q", c.args[0], "%7")
		}
		if c.args[2] != "/tmp/pane-popup" {
			t.Errorf("DisplayPopup dir = %q, want %q", c.args[2], "/tmp/pane-popup")
		}
	}
	if !found {
		t.Error("expected DisplayPopup to be called")
	}
}

func TestManager_PaneCapture_ReturnsContent(t *testing.T) {
	mgr, mock, _ := newTestManager(t)
	sess := paneTestSession(t, mgr, "/tmp/pane-cap", "%9", "sess-cap")
	mock.captured["%9"] = "line1\nline2"

	got, err := mgr.PaneCapture(sess.ID, false)
	if err != nil {
		t.Fatalf("PaneCapture failed: %v", err)
	}
	if got != "line1\nline2" {
		t.Errorf("PaneCapture = %q, want %q", got, "line1\nline2")
	}
}

func TestManager_PaneSendKeys_LiteralVsNamed(t *testing.T) {
	mgr, mock, _ := newTestManager(t)
	sess := paneTestSession(t, mgr, "/tmp/pane-keys", "%11", "sess-keys")

	if err := mgr.PaneSendKeys(sess.ID, "hello", true); err != nil {
		t.Fatalf("PaneSendKeys literal failed: %v", err)
	}
	if !mock.hasCalledWith("SendKeysLiteral", "%11") {
		t.Error("expected SendKeysLiteral for literal=true")
	}

	if err := mgr.PaneSendKeys(sess.ID, "Enter", false); err != nil {
		t.Fatalf("PaneSendKeys named failed: %v", err)
	}
	if !mock.hasCalledWith("SendKeys", "%11") {
		t.Error("expected SendKeys for literal=false")
	}
}

func TestManager_PaneSplit_DelegatesToTmux(t *testing.T) {
	mgr, mock, _ := newTestManager(t)
	sess := paneTestSession(t, mgr, "/tmp/pane-split", "%13", "sess-split")

	if err := mgr.PaneSplit(sess.ID, "top", true, 30); err != nil {
		t.Fatalf("PaneSplit failed: %v", err)
	}
	if !mock.hasCalledWith("SplitWindow", "%13") {
		t.Error("expected SplitWindow to be called with pane target")
	}
}

func TestManager_Pane_TmuxUnavailable(t *testing.T) {
	mgr, _, _ := newTestManager(t)
	sess := paneTestSession(t, mgr, "/tmp/pane-nil", "%15", "sess-nil")
	mgr.SetTmuxClient(nil)

	if err := mgr.PanePopup(sess.ID, "cmd", "", "", ""); err == nil {
		t.Error("PanePopup: expected error when tmux is nil")
	}
	if err := mgr.PaneSplit(sess.ID, "cmd", false, 50); err == nil {
		t.Error("PaneSplit: expected error when tmux is nil")
	}
	if _, err := mgr.PaneCapture(sess.ID, false); err == nil {
		t.Error("PaneCapture: expected error when tmux is nil")
	}
	if err := mgr.PaneSendKeys(sess.ID, "x", true); err == nil {
		t.Error("PaneSendKeys: expected error when tmux is nil")
	}
}
