package host

import (
	"sort"
	"testing"

	"github.com/takaaki-s/claude-code-valet/internal/config"
	"github.com/takaaki-s/claude-code-valet/internal/notify"
	"github.com/takaaki-s/claude-code-valet/internal/session"
)

// mockSlaveClient implements SlaveClient for testing.
type mockSlaveClient struct {
	running  bool
	sessions []session.Info
	entries  []notify.Entry
}

func (m *mockSlaveClient) IsRunning() bool                         { return m.running }
func (m *mockSlaveClient) ListWithHostID() ([]session.Info, error) { return m.sessions, nil }
func (m *mockSlaveClient) NotificationHistoryWithHostID() ([]notify.Entry, error) {
	return m.entries, nil
}
func (m *mockSlaveClient) SendRaw(action string, data, visited []byte) ([]byte, error) {
	return []byte(`{"success":true}`), nil
}

// twoSSHHostConfigs returns a pair of SSH host configs for use in tests.
func twoSSHHostConfigs() []config.HostConfig {
	return []config.HostConfig{
		{ID: "ec2", Type: "ssh", Host: "ec2-host"},
		{ID: "docker-dev", Type: "ssh", Host: "docker-host"},
	}
}

func TestRegistry_NewWithLocalHost(t *testing.T) {
	r := NewRegistry(nil)

	local := r.Local()
	if local == nil {
		t.Fatal("Local() returned nil for empty config")
	}
	if local.ID != "local" {
		t.Errorf("Local().ID = %q, want %q", local.ID, "local")
	}
	if local.Type != "local" {
		t.Errorf("Local().Type = %q, want %q", local.Type, "local")
	}
}

func TestRegistry_NewWithRemoteHosts(t *testing.T) {
	r := NewRegistry(twoSSHHostConfigs())

	all := r.All()
	if len(all) != 3 {
		t.Fatalf("All() returned %d hosts, want 3", len(all))
	}

	remotes := r.Remotes()
	if len(remotes) != 2 {
		t.Fatalf("Remotes() returned %d hosts, want 2", len(remotes))
	}
}

func TestRegistry_Get_Local(t *testing.T) {
	r := NewRegistry(nil)

	h, ok := r.Get("local")
	if !ok {
		t.Fatal("Get(\"local\") returned ok=false")
	}
	if h.ID != "local" {
		t.Errorf("Get(\"local\").ID = %q, want %q", h.ID, "local")
	}
}

func TestRegistry_Get_Remote(t *testing.T) {
	r := NewRegistry(twoSSHHostConfigs())

	h, ok := r.Get("ec2")
	if !ok {
		t.Fatal("Get(\"ec2\") returned ok=false")
	}
	if h.ID != "ec2" {
		t.Errorf("Get(\"ec2\").ID = %q, want %q", h.ID, "ec2")
	}
	if h.Type != "ssh" {
		t.Errorf("Get(\"ec2\").Type = %q, want %q", h.Type, "ssh")
	}
}

func TestRegistry_Get_NotFound(t *testing.T) {
	r := NewRegistry(nil)

	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("Get(\"nonexistent\") returned ok=true, want false")
	}
}

func TestRegistry_All_LocalFirst(t *testing.T) {
	r := NewRegistry(twoSSHHostConfigs())

	all := r.All()
	if len(all) == 0 {
		t.Fatal("All() returned empty slice")
	}
	if all[0].ID != "local" {
		t.Errorf("All()[0].ID = %q, want %q (local should be first)", all[0].ID, "local")
	}
}

func TestRegistry_Remotes(t *testing.T) {
	r := NewRegistry(twoSSHHostConfigs())

	remotes := r.Remotes()
	for _, h := range remotes {
		if h.ID == "local" {
			t.Error("Remotes() contains local host, should only contain remote hosts")
		}
	}

	// Collect remote IDs and verify both are present
	ids := make(map[string]bool)
	for _, h := range remotes {
		ids[h.ID] = true
	}
	if !ids["ec2"] {
		t.Error("Remotes() missing host \"ec2\"")
	}
	if !ids["docker-dev"] {
		t.Error("Remotes() missing host \"docker-dev\"")
	}
}

func TestRegistry_AllIDs(t *testing.T) {
	r := NewRegistry(twoSSHHostConfigs())

	ids := r.AllIDs()
	sort.Strings(ids)

	want := []string{"docker-dev", "ec2", "local"}
	if len(ids) != len(want) {
		t.Fatalf("AllIDs() returned %d IDs, want %d", len(ids), len(want))
	}
	for i, id := range ids {
		if id != want[i] {
			t.Errorf("AllIDs()[%d] = %q, want %q", i, id, want[i])
		}
	}
}

func TestRegistry_SetClient(t *testing.T) {
	r := NewRegistry(twoSSHHostConfigs())

	client := &mockSlaveClient{running: true}
	r.SetClient("ec2", client)

	h, ok := r.Get("ec2")
	if !ok {
		t.Fatal("Get(\"ec2\") returned ok=false after SetClient")
	}
	if h.Client == nil {
		t.Fatal("Client is nil after SetClient")
	}
	if !h.Client.IsRunning() {
		t.Error("Client.IsRunning() = false, want true")
	}
}

func TestRegistry_IsConnected_True(t *testing.T) {
	r := NewRegistry(twoSSHHostConfigs())

	client := &mockSlaveClient{running: true}
	r.SetClient("ec2", client)

	if !r.IsConnected("ec2") {
		t.Error("IsConnected(\"ec2\") = false after SetClient with running client, want true")
	}
}

func TestRegistry_IsConnected_False(t *testing.T) {
	r := NewRegistry(twoSSHHostConfigs())

	if r.IsConnected("ec2") {
		t.Error("IsConnected(\"ec2\") = true before SetClient, want false")
	}
}

func TestRegistry_IsConnected_NotFound(t *testing.T) {
	r := NewRegistry(nil)

	if r.IsConnected("nonexistent") {
		t.Error("IsConnected(\"nonexistent\") = true for unknown host, want false")
	}
}
