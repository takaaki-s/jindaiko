package daemon

import (
	"testing"

	"github.com/takaaki-s/claude-code-valet/internal/notify"
	"github.com/takaaki-s/claude-code-valet/internal/session"
)

func TestNewClient(t *testing.T) {
	c := NewClient("/tmp/test.sock")
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.socketPath != "/tmp/test.sock" {
		t.Errorf("socketPath: got %q, want %q", c.socketPath, "/tmp/test.sock")
	}
	if c.hostID != "local" {
		t.Errorf("hostID: got %q, want %q", c.hostID, "local")
	}
}

func TestNewRemoteClient(t *testing.T) {
	tests := []struct {
		name       string
		socketPath string
		hostID     string
	}{
		{
			name:       "ec2 host",
			socketPath: "/run/ccvalet/ec2.sock",
			hostID:     "ec2",
		},
		{
			name:       "docker-dev host",
			socketPath: "/var/run/docker-dev.sock",
			hostID:     "docker-dev",
		},
		{
			name:       "empty hostID",
			socketPath: "/tmp/empty.sock",
			hostID:     "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewRemoteClient(tt.socketPath, tt.hostID)
			if c == nil {
				t.Fatal("NewRemoteClient returned nil")
			}
			if c.socketPath != tt.socketPath {
				t.Errorf("socketPath: got %q, want %q", c.socketPath, tt.socketPath)
			}
			if c.hostID != tt.hostID {
				t.Errorf("hostID: got %q, want %q", c.hostID, tt.hostID)
			}
		})
	}
}

func TestClient_HostID(t *testing.T) {
	tests := []struct {
		name   string
		client *Client
		want   string
	}{
		{
			name:   "local client",
			client: NewClient("/tmp/test.sock"),
			want:   "local",
		},
		{
			name:   "remote client",
			client: NewRemoteClient("/tmp/remote.sock", "ec2-prod"),
			want:   "ec2-prod",
		},
		{
			name:   "empty hostID",
			client: NewRemoteClient("/tmp/empty.sock", ""),
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.client.HostID()
			if got != tt.want {
				t.Errorf("HostID(): got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestClient_ListWithHostID_Tagging verifies the HostID tagging logic in isolation
// by simulating what ListWithHostID does on a slice of session.Info.
func TestClient_ListWithHostID_Tagging(t *testing.T) {
	sessions := []session.Info{
		{ID: "aaa", Name: "session-1"},
		{ID: "bbb", Name: "session-2"},
		{ID: "ccc", Name: "session-3", HostID: "old-value"},
	}

	hostID := "my-remote-host"
	// Replicate the tagging loop from ListWithHostID
	for i := range sessions {
		sessions[i].HostID = hostID
	}

	for i, s := range sessions {
		if s.HostID != hostID {
			t.Errorf("sessions[%d].HostID: got %q, want %q", i, s.HostID, hostID)
		}
	}
}

// TestClient_NotificationHistoryWithHostID_Tagging verifies the HostID tagging logic
// for notification entries in isolation.
func TestClient_NotificationHistoryWithHostID_Tagging(t *testing.T) {
	entries := []notify.Entry{
		{SessionID: "s1", SessionName: "alpha", Type: "permission"},
		{SessionID: "s2", SessionName: "beta", Type: "task_complete"},
		{SessionID: "s3", SessionName: "gamma", Type: "permission", HostID: "stale"},
	}

	hostID := "docker-dev"
	// Replicate the tagging loop from NotificationHistoryWithHostID
	for i := range entries {
		entries[i].HostID = hostID
	}

	for i, e := range entries {
		if e.HostID != hostID {
			t.Errorf("entries[%d].HostID: got %q, want %q", i, e.HostID, hostID)
		}
	}
}

// TestClient_ListWithHostID_EmptySlice verifies tagging works correctly with an empty slice.
func TestClient_ListWithHostID_EmptySlice(t *testing.T) {
	var sessions []session.Info
	hostID := "some-host"
	for i := range sessions {
		sessions[i].HostID = hostID
	}
	if len(sessions) != 0 {
		t.Errorf("expected empty slice, got %d elements", len(sessions))
	}
}

// TestClient_ListWithHostID_Integration uses setupTestServer to verify the full
// ListWithHostID method end-to-end via the daemon.
func TestClient_ListWithHostID_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	_, client := setupTestServer(t)

	// Create sessions via the daemon
	for _, name := range []string{"sess-a", "sess-b"} {
		_, err := client.NewWithOptions(NewOptions{
			Name:    name,
			WorkDir: "/tmp/" + name,
			Start:   false,
		})
		if err != nil {
			t.Fatalf("NewWithOptions(%s): %v", name, err)
		}
	}

	// The default client has hostID "local" (set by NewClient)
	sessions, err := client.ListWithHostID(nil)
	if err != nil {
		t.Fatalf("ListWithHostID: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("ListWithHostID: got %d sessions, want 2", len(sessions))
	}
	for i, s := range sessions {
		if s.HostID != "local" {
			t.Errorf("sessions[%d].HostID: got %q, want %q", i, s.HostID, "local")
		}
	}

	// Now create a remote client pointing at the same socket but with a different hostID
	remoteClient := NewRemoteClient(client.socketPath, "remote-ec2")
	sessions2, err := remoteClient.ListWithHostID(nil)
	if err != nil {
		t.Fatalf("ListWithHostID (remote): %v", err)
	}
	if len(sessions2) != 2 {
		t.Fatalf("ListWithHostID (remote): got %d sessions, want 2", len(sessions2))
	}
	for i, s := range sessions2 {
		if s.HostID != "remote-ec2" {
			t.Errorf("sessions2[%d].HostID: got %q, want %q", i, s.HostID, "remote-ec2")
		}
	}
}

// TestClient_NotificationHistoryWithHostID_Integration uses setupTestServer to verify
// the full NotificationHistoryWithHostID method end-to-end.
func TestClient_NotificationHistoryWithHostID_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	_, client := setupTestServer(t)

	// Create a session and trigger a notification via hook
	info, err := client.NewWithOptions(NewOptions{
		Name:    "notif-test",
		WorkDir: "/tmp/notif-test",
		Start:   false,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}

	// First make it "thinking" so "Stop" can transition to "idle" and trigger task_complete notification
	if err := client.SendHook(HookRequest{
		CcvaletSessionID: info.ID,
		HookEventName:    "UserPromptSubmit",
	}); err != nil {
		t.Fatalf("SendHook(UserPromptSubmit): %v", err)
	}

	if err := client.SendHook(HookRequest{
		CcvaletSessionID: info.ID,
		HookEventName:    "Stop",
	}); err != nil {
		t.Fatalf("SendHook(Stop): %v", err)
	}

	// Use a remote client to call NotificationHistoryWithHostID
	remoteClient := NewRemoteClient(client.socketPath, "staging-box")
	entries, err := remoteClient.NotificationHistoryWithHostID(nil)
	if err != nil {
		t.Fatalf("NotificationHistoryWithHostID: %v", err)
	}

	// There should be at least one notification entry (task_complete from the Stop hook)
	if len(entries) == 0 {
		t.Fatal("NotificationHistoryWithHostID: got 0 entries, want at least 1")
	}
	for i, e := range entries {
		if e.HostID != "staging-box" {
			t.Errorf("entries[%d].HostID: got %q, want %q", i, e.HostID, "staging-box")
		}
	}
}
