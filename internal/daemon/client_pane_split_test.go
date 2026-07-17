package daemon

import (
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
)

// fakeDaemon replies to every incoming request with the same canned Response,
// so client-side wire handling can be tested against arbitrary payload shapes
// (including responses produced by older daemon versions).
func fakeDaemon(t *testing.T, reply Response) string {
	t.Helper()
	sockPath := filepath.Join(t.TempDir(), "daemon.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				var req Request
				_ = json.NewDecoder(c).Decode(&req)
				_ = json.NewEncoder(c).Encode(reply)
			}(conn)
		}
	}()
	return sockPath
}

// TestPaneSplit_EmptyDataIsNotAnError guards the version-mismatch case: a
// pre-PR-#107 daemon replies with `{"success":true}` (no data). The client
// must treat that as a successful split with an unknown pane ID, not the
// confusing "unexpected end of JSON input" json.Unmarshal produces on empty
// bytes.
func TestPaneSplit_EmptyDataIsNotAnError(t *testing.T) {
	sock := fakeDaemon(t, Response{Success: true})
	c := NewClient(sock)

	paneID, err := c.PaneSplit(PaneSplitRequest{ID: "sess"})
	if err != nil {
		t.Fatalf("PaneSplit returned error on empty Data: %v", err)
	}
	if paneID != "" {
		t.Errorf("paneID = %q, want empty for a data-less success", paneID)
	}
}

// TestPaneSplit_ParsesPaneIDFromData covers the normal path: a fresh daemon
// returns the pane ID inside Data, and the client hands it back to the caller.
func TestPaneSplit_ParsesPaneIDFromData(t *testing.T) {
	respData, _ := json.Marshal(PaneSplitResponse{PaneID: "%42"})
	sock := fakeDaemon(t, Response{Success: true, Data: respData})
	c := NewClient(sock)

	paneID, err := c.PaneSplit(PaneSplitRequest{ID: "sess"})
	if err != nil {
		t.Fatalf("PaneSplit: %v", err)
	}
	if paneID != "%42" {
		t.Errorf("paneID = %q, want %q", paneID, "%42")
	}
}

// TestPaneSplit_PropagatesDaemonError makes sure daemon-side failures still
// surface as errors even after the empty-Data tolerance above.
func TestPaneSplit_PropagatesDaemonError(t *testing.T) {
	sock := fakeDaemon(t, Response{Success: false, Error: "session not found: sess"})
	c := NewClient(sock)

	_, err := c.PaneSplit(PaneSplitRequest{ID: "sess"})
	if err == nil {
		t.Fatal("expected error from failed daemon response, got nil")
	}
	if err.Error() != "session not found: sess" {
		t.Errorf("err = %q, want %q", err.Error(), "session not found: sess")
	}
}
