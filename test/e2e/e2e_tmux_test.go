//go:build e2e

package e2e

import (
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/takaaki-s/claude-code-valet/internal/config"
	"github.com/takaaki-s/claude-code-valet/internal/daemon"
	"github.com/takaaki-s/claude-code-valet/internal/session"
	"github.com/takaaki-s/claude-code-valet/internal/tmux"
)

// --- helpers ---

// setupE2EWithDataDir creates a daemon server using the provided data/config dirs.
// Returns the client and server (server is needed for Stop in recovery tests).
func setupE2EWithDataDir(t *testing.T, dataDir, configDir string) (*daemon.Client, *daemon.Server) {
	t.Helper()

	socketPath := filepath.Join(t.TempDir(), "e2e-tmux.sock")

	server, err := daemon.NewServer(socketPath, dataDir, configDir)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	go func() {
		_ = server.Start()
	}()

	// Wait for socket
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("unix", socketPath)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Cleanup(func() {
		server.Stop()
	})

	return daemon.NewClient(socketPath), server
}

// hasTmuxSession checks if a tmux session exists on the ccvalet socket.
func hasTmuxSession(name string) bool {
	tc, err := tmux.NewClient()
	if err != nil {
		return false
	}
	return tc.HasSession(name)
}

// cleanupTmuxSessions kills all sessions on the ccvalet tmux socket.
func cleanupTmuxSessions(t *testing.T) {
	t.Helper()
	_ = exec.Command("tmux", "-L", tmux.SocketName, "kill-server").Run()
}

// waitForStatus polls client.List until the session reaches the expected status or times out.
func waitForStatus(t *testing.T, client *daemon.Client, sessionID string, want session.Status, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		sessions, err := client.List()
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		for _, s := range sessions {
			if s.ID == sessionID && s.Status == want {
				return
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	// Show actual status on timeout
	sessions, _ := client.List()
	for _, s := range sessions {
		if s.ID == sessionID {
			t.Fatalf("timed out waiting for session %s to reach status %q (current: %q)", sessionID, want, s.Status)
		}
	}
	t.Fatalf("timed out: session %s not found in list", sessionID)
}

// --- tests ---

func TestE2E_TmuxSessionCreation(t *testing.T) {
	t.Cleanup(func() { cleanupTmuxSessions(t) })

	client := setupE2E(t)

	info, err := client.NewWithOptions(daemon.NewOptions{
		Name:    "tmux-create",
		WorkDir: t.TempDir(),
		Start:   true,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}

	innerName := tmux.InnerSessionName(info.ID)

	// tmux session should exist on the ccvalet socket
	// Wait briefly for async session creation
	time.Sleep(500 * time.Millisecond)

	if !hasTmuxSession(innerName) {
		t.Fatalf("tmux session %q should exist after Start:true", innerName)
	}

	// Inner session name should follow the naming convention
	if innerName != "sess-"+info.ID {
		t.Errorf("InnerSessionName = %q, want %q", innerName, "sess-"+info.ID)
	}
}

func TestE2E_KillWithTmuxCleanup(t *testing.T) {
	t.Cleanup(func() { cleanupTmuxSessions(t) })

	client := setupE2E(t)

	info, err := client.NewWithOptions(daemon.NewOptions{
		Name:    "tmux-kill",
		WorkDir: t.TempDir(),
		Start:   true,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}

	innerName := tmux.InnerSessionName(info.ID)
	time.Sleep(500 * time.Millisecond)

	if !hasTmuxSession(innerName) {
		t.Fatal("tmux session should exist before Kill")
	}

	// Kill
	if err := client.Kill(info.ID, ""); err != nil {
		t.Fatalf("Kill: %v", err)
	}

	// tmux session should be gone
	time.Sleep(200 * time.Millisecond)
	if hasTmuxSession(innerName) {
		t.Error("tmux session should not exist after Kill")
	}

	// Status should be stopped
	sessions, err := client.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Status != session.StatusStopped {
		t.Errorf("Status after Kill: got %q, want %q", sessions[0].Status, session.StatusStopped)
	}
}

func TestE2E_DeleteWithTmuxCleanup(t *testing.T) {
	t.Cleanup(func() { cleanupTmuxSessions(t) })

	client := setupE2E(t)

	info, err := client.NewWithOptions(daemon.NewOptions{
		Name:    "tmux-delete",
		WorkDir: t.TempDir(),
		Start:   true,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}

	innerName := tmux.InnerSessionName(info.ID)
	time.Sleep(500 * time.Millisecond)

	if !hasTmuxSession(innerName) {
		t.Fatal("tmux session should exist before Delete")
	}

	// Delete
	if err := client.Delete(info.ID, ""); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// tmux session should be gone
	time.Sleep(200 * time.Millisecond)
	if hasTmuxSession(innerName) {
		t.Error("tmux session should not exist after Delete")
	}

	// Session should be removed from list
	sessions, err := client.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions after Delete, got %d", len(sessions))
	}
}

func TestE2E_SessionDataPersistence(t *testing.T) {
	t.Cleanup(func() { cleanupTmuxSessions(t) })

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "sessions")
	configDir := filepath.Join(tmpDir, "config")
	client, _ := setupE2EWithDataDir(t, dataDir, configDir)

	workDir := t.TempDir()
	info, err := client.NewWithOptions(daemon.NewOptions{
		Name:    "persist-test",
		WorkDir: workDir,
		Start:   true,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}

	// JSON file should exist on disk
	jsonPath := filepath.Join(dataDir, info.ID+".json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("session JSON file not found: %v", err)
	}

	// Decode and verify fields
	var persisted map[string]interface{}
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if persisted["id"] != info.ID {
		t.Errorf("persisted ID = %v, want %q", persisted["id"], info.ID)
	}
	if persisted["name"] != "persist-test" {
		t.Errorf("persisted name = %v, want %q", persisted["name"], "persist-test")
	}
	if persisted["work_dir"] != workDir {
		t.Errorf("persisted work_dir = %v, want %q", persisted["work_dir"], workDir)
	}

	// Delete should remove the file
	if err := client.Delete(info.ID, ""); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(jsonPath); err == nil {
		t.Error("session JSON file should be removed after Delete")
	}
}

func TestE2E_SessionRecovery(t *testing.T) {
	t.Cleanup(func() { cleanupTmuxSessions(t) })

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "sessions")
	configDir := filepath.Join(tmpDir, "config")

	// Phase 1: Start server and create a session
	client, server := setupE2EWithDataDir(t, dataDir, configDir)

	info, err := client.NewWithOptions(daemon.NewOptions{
		Name:    "recovery-test",
		WorkDir: t.TempDir(),
		Start:   true,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}

	innerName := tmux.InnerSessionName(info.ID)
	time.Sleep(500 * time.Millisecond)

	if !hasTmuxSession(innerName) {
		t.Fatal("tmux session should exist after Start")
	}

	// Phase 2: Stop the server (simulating daemon restart)
	server.Stop()

	// tmux session should still exist (daemon stop doesn't kill inner sessions)
	if !hasTmuxSession(innerName) {
		t.Fatal("tmux session should survive daemon stop")
	}

	// Phase 3: Create new Manager from same data directory (simulating daemon restart)
	configMgr, err := config.NewManager(configDir)
	if err != nil {
		t.Fatalf("NewConfigManager: %v", err)
	}

	mgr, err := session.NewManager(dataDir, configDir, configMgr)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Set real tmux client and recover
	tc, err := tmux.NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	mgr.SetTmuxClient(tc)
	mgr.RecoverTmuxSessions()

	// Verify session is recovered
	recovered := mgr.List()
	if len(recovered) != 1 {
		t.Fatalf("expected 1 recovered session, got %d", len(recovered))
	}
	if recovered[0].ID != info.ID {
		t.Errorf("recovered ID = %q, want %q", recovered[0].ID, info.ID)
	}
	if recovered[0].Name != "recovery-test" {
		t.Errorf("recovered Name = %q, want %q", recovered[0].Name, "recovery-test")
	}
}

func TestE2E_MultipleSessionsTmux(t *testing.T) {
	t.Cleanup(func() { cleanupTmuxSessions(t) })

	client := setupE2E(t)

	// Create 3 sessions
	type sess struct {
		id        string
		innerName string
	}
	sessions := make([]sess, 3)
	for i := range 3 {
		info, err := client.NewWithOptions(daemon.NewOptions{
			Name:    filepath.Base(t.TempDir()),
			WorkDir: t.TempDir(),
			Start:   true,
		})
		if err != nil {
			t.Fatalf("NewWithOptions(%d): %v", i, err)
		}
		sessions[i] = sess{id: info.ID, innerName: tmux.InnerSessionName(info.ID)}
	}

	time.Sleep(500 * time.Millisecond)

	// All 3 tmux sessions should exist
	for i, s := range sessions {
		if !hasTmuxSession(s.innerName) {
			t.Fatalf("tmux session %d (%s) should exist", i, s.innerName)
		}
	}

	// Kill the middle one
	if err := client.Kill(sessions[1].id, ""); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	// Middle should be gone, others still alive
	if hasTmuxSession(sessions[1].innerName) {
		t.Error("killed session's tmux should not exist")
	}
	if !hasTmuxSession(sessions[0].innerName) {
		t.Error("session 0 tmux should still exist after killing session 1")
	}
	if !hasTmuxSession(sessions[2].innerName) {
		t.Error("session 2 tmux should still exist after killing session 1")
	}

	// Delete the rest
	for _, i := range []int{0, 2} {
		if err := client.Delete(sessions[i].id, ""); err != nil {
			t.Fatalf("Delete(%d): %v", i, err)
		}
	}
	time.Sleep(200 * time.Millisecond)

	// All tmux sessions should be gone
	for i, s := range sessions {
		if hasTmuxSession(s.innerName) {
			t.Errorf("tmux session %d (%s) should not exist after cleanup", i, s.innerName)
		}
	}
}

func TestE2E_HookCWDUpdateOnStartedSession(t *testing.T) {
	t.Cleanup(func() { cleanupTmuxSessions(t) })

	client := setupE2E(t)

	info, err := client.NewWithOptions(daemon.NewOptions{
		Name:    "cwd-update",
		WorkDir: t.TempDir(),
		Start:   true,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}

	// Send hook with CWD
	newCWD := t.TempDir()
	if err := client.SendHook(daemon.HookRequest{
		CcvaletSessionID: info.ID,
		HookEventName:    "UserPromptSubmit",
		CWD:              newCWD,
	}); err != nil {
		t.Fatalf("SendHook: %v", err)
	}

	// Verify CWD is updated
	sessions, err := client.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].CurrentWorkDir != newCWD {
		t.Errorf("CurrentWorkDir = %q, want %q", sessions[0].CurrentWorkDir, newCWD)
	}
	if sessions[0].Status != session.StatusThinking {
		t.Errorf("Status = %q, want %q", sessions[0].Status, session.StatusThinking)
	}
}
