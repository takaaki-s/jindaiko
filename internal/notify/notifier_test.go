package notify

import (
	"testing"
	"time"
)

func TestNewNotifier(t *testing.T) {
	n := NewNotifier()

	if !n.enabled {
		t.Error("expected enabled to be true by default")
	}
	if n.debounceInterval != 3*time.Second {
		t.Errorf("debounceInterval: got %v, want %v", n.debounceInterval, 3*time.Second)
	}
	if n.history == nil {
		t.Fatal("expected history to be initialized, got nil")
	}
	if n.lastNotify == nil {
		t.Fatal("expected lastNotify map to be initialized, got nil")
	}
	if len(n.lastNotify) != 0 {
		t.Errorf("lastNotify should be empty, got %d entries", len(n.lastNotify))
	}
}

func TestNotifier_SetEnabled(t *testing.T) {
	n := NewNotifier()

	n.SetEnabled(false)
	n.mu.Lock()
	got := n.enabled
	n.mu.Unlock()
	if got {
		t.Error("expected enabled to be false after SetEnabled(false)")
	}

	n.SetEnabled(true)
	n.mu.Lock()
	got = n.enabled
	n.mu.Unlock()
	if !got {
		t.Error("expected enabled to be true after SetEnabled(true)")
	}
}

func TestNotifier_NotifyPermission(t *testing.T) {
	n := NewNotifier()
	// Disable to prevent actual notification sending
	n.SetEnabled(false)

	// Re-enable just the enabled flag
	n.mu.Lock()
	n.enabled = true
	n.mu.Unlock()

	n.NotifyPermission("sess-1", "my-session")

	entries := n.NotificationHistory()
	if len(entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Type != "permission" {
		t.Errorf("Type: got %q, want %q", entry.Type, "permission")
	}
	if entry.SessionID != "sess-1" {
		t.Errorf("SessionID: got %q, want %q", entry.SessionID, "sess-1")
	}
	if entry.SessionName != "my-session" {
		t.Errorf("SessionName: got %q, want %q", entry.SessionName, "my-session")
	}
	if entry.Message == "" {
		t.Error("expected Message to be non-empty")
	}
	if entry.Timestamp.IsZero() {
		t.Error("expected Timestamp to be set")
	}
}

func TestNotifier_NotifyTaskComplete(t *testing.T) {
	n := NewNotifier()

	n.NotifyTaskComplete("sess-2", "build-session")

	entries := n.NotificationHistory()
	if len(entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Type != "task_complete" {
		t.Errorf("Type: got %q, want %q", entry.Type, "task_complete")
	}
	if entry.SessionID != "sess-2" {
		t.Errorf("SessionID: got %q, want %q", entry.SessionID, "sess-2")
	}
	if entry.SessionName != "build-session" {
		t.Errorf("SessionName: got %q, want %q", entry.SessionName, "build-session")
	}
	if entry.Message == "" {
		t.Error("expected Message to be non-empty")
	}
}

func TestNotifier_NotificationHistory(t *testing.T) {
	n := NewNotifier()

	n.NotifyPermission("s1", "session-alpha")
	n.NotifyTaskComplete("s2", "session-beta")

	entries := n.NotificationHistory()
	if len(entries) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(entries))
	}

	// History returns newest first, so task_complete (added second) should be first
	types := make(map[string]bool)
	ids := make(map[string]bool)
	for _, e := range entries {
		types[e.Type] = true
		ids[e.SessionID] = true
	}

	if !types["permission"] {
		t.Error("expected a 'permission' entry in history")
	}
	if !types["task_complete"] {
		t.Error("expected a 'task_complete' entry in history")
	}
	if !ids["s1"] {
		t.Error("expected entry with SessionID 's1' in history")
	}
	if !ids["s2"] {
		t.Error("expected entry with SessionID 's2' in history")
	}
}

func TestNotifier_Debounce(t *testing.T) {
	n := NewNotifier()
	// Set a long debounce interval so the second call is definitely within it
	n.mu.Lock()
	n.debounceInterval = 10 * time.Second
	n.mu.Unlock()

	// First call should update lastNotify
	n.notify("sess-1", "Permission Required", "msg1")

	n.mu.Lock()
	key := "sess-1:Permission Required"
	firstTime, ok := n.lastNotify[key]
	n.mu.Unlock()

	if !ok {
		t.Fatal("expected lastNotify to have an entry after first notify call")
	}
	if firstTime.IsZero() {
		t.Fatal("expected lastNotify timestamp to be non-zero")
	}

	// Second call with same key should be debounced (lastNotify not updated)
	n.notify("sess-1", "Permission Required", "msg2")

	n.mu.Lock()
	secondTime := n.lastNotify[key]
	n.mu.Unlock()

	if !secondTime.Equal(firstTime) {
		t.Errorf("expected lastNotify timestamp to remain unchanged after debounced call: first=%v, second=%v",
			firstTime, secondTime)
	}

	// Different key should NOT be debounced
	n.notify("sess-2", "Permission Required", "msg3")

	n.mu.Lock()
	key2 := "sess-2:Permission Required"
	_, ok2 := n.lastNotify[key2]
	n.mu.Unlock()

	if !ok2 {
		t.Error("expected lastNotify to have an entry for a different session key")
	}
}

func TestNotifier_Disabled(t *testing.T) {
	n := NewNotifier()

	n.SetEnabled(false)

	n.NotifyPermission("sess-disabled", "disabled-session")

	// History should still have the entry (history.Add is called before notify)
	entries := n.NotificationHistory()
	if len(entries) != 1 {
		t.Fatalf("expected 1 history entry even when disabled, got %d", len(entries))
	}
	if entries[0].SessionID != "sess-disabled" {
		t.Errorf("SessionID: got %q, want %q", entries[0].SessionID, "sess-disabled")
	}
	if entries[0].Type != "permission" {
		t.Errorf("Type: got %q, want %q", entries[0].Type, "permission")
	}

	// lastNotify should NOT be updated when disabled (notify returns early)
	n.mu.Lock()
	key := "sess-disabled:Permission Required"
	_, ok := n.lastNotify[key]
	n.mu.Unlock()

	if ok {
		t.Error("expected lastNotify to NOT have an entry when notifier is disabled")
	}
}
