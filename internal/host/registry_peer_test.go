package host

import (
	"testing"

	"github.com/takaaki-s/honjin/internal/config"
)

func TestRegistry_RegisterPeer(t *testing.T) {
	r := NewRegistry(nil)

	client := &mockSlaveClient{running: true}
	r.RegisterPeer("mac", "ssh", client)

	h, ok := r.Get("mac")
	if !ok {
		t.Fatal("peer not found after RegisterPeer")
	}
	if !h.IsPeer {
		t.Error("IsPeer should be true")
	}
	if h.Type != "ssh" {
		t.Errorf("Type = %q, want ssh", h.Type)
	}
	if h.Client == nil {
		t.Error("Client should not be nil")
	}
}

func TestRegistry_Peers(t *testing.T) {
	r := NewRegistry([]config.HostConfig{
		{ID: "ec2", Type: "ssh"},
	})

	// No peers initially
	peers := r.Peers()
	if len(peers) != 0 {
		t.Errorf("Peers() = %d, want 0 (no peers registered)", len(peers))
	}

	// Register a peer
	r.RegisterPeer("mac", "ssh", &mockSlaveClient{running: true})

	peers = r.Peers()
	if len(peers) != 1 {
		t.Fatalf("Peers() = %d, want 1", len(peers))
	}
	if peers[0].ID != "mac" {
		t.Errorf("peer ID = %q, want mac", peers[0].ID)
	}
}

func TestRegistry_AllReachable(t *testing.T) {
	r := NewRegistry([]config.HostConfig{
		{ID: "ec2", Type: "ssh"},
	})

	// Only remote, no peers
	reachable := r.AllReachable()
	if len(reachable) != 1 {
		t.Fatalf("AllReachable() = %d, want 1", len(reachable))
	}

	// Add a peer
	r.RegisterPeer("mac", "ssh", &mockSlaveClient{running: true})

	reachable = r.AllReachable()
	if len(reachable) != 2 {
		t.Fatalf("AllReachable() = %d, want 2", len(reachable))
	}

	// Verify local is not included
	for _, h := range reachable {
		if h.ID == "local" {
			t.Error("AllReachable() should not include local")
		}
	}
}

func TestRegistry_AllReachable_ExcludesLocal(t *testing.T) {
	r := NewRegistry(nil)

	reachable := r.AllReachable()
	if len(reachable) != 0 {
		t.Errorf("AllReachable() = %d, want 0 (only local)", len(reachable))
	}
}

func TestRegistry_Remotes_ExcludesPeers(t *testing.T) {
	r := NewRegistry([]config.HostConfig{
		{ID: "ec2", Type: "ssh"},
	})
	r.RegisterPeer("mac", "ssh", &mockSlaveClient{running: true})

	remotes := r.Remotes()
	if len(remotes) != 1 {
		t.Fatalf("Remotes() = %d, want 1 (peers excluded)", len(remotes))
	}
	if remotes[0].ID != "ec2" {
		t.Errorf("remote ID = %q, want ec2", remotes[0].ID)
	}
}
