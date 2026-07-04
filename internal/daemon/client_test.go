package daemon

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient("/tmp/test.sock")
	if c.socketPath != "/tmp/test.sock" {
		t.Errorf("socketPath: got %q, want %q", c.socketPath, "/tmp/test.sock")
	}
}
