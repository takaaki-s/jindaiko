package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStore_NewCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sessions")
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if store == nil {
		t.Fatal("NewStore returned nil")
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}
}

func TestStore_SaveAndLoad_RoundTrip(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	now := time.Now().Truncate(time.Millisecond)
	session := &Session{
		ID:              "test-123",
		Name:            "my-session",
		WorkDir:         "/home/user/project",
		CreatedAt:       now,
		Status:          StatusIdle,
		LastActiveAt:    now,
		ErrorMessage:    "some error",
		ClaudeSessionID: "claude-456",
		HostID:          "local",
		TmuxWindowName:  "sess-test-123",
		TmuxPaneID:      "%42",
		CurrentWorkDir:  "/runtime/dir", // persisted
		CurrentBranch:   "main",         // json:"-"
	}

	if err := store.Save(session); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load("test-123")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.ID != session.ID {
		t.Errorf("ID: got %q, want %q", loaded.ID, session.ID)
	}
	if loaded.Name != session.Name {
		t.Errorf("Name: got %q, want %q", loaded.Name, session.Name)
	}
	if loaded.WorkDir != session.WorkDir {
		t.Errorf("WorkDir: got %q, want %q", loaded.WorkDir, session.WorkDir)
	}
	if loaded.Status != session.Status {
		t.Errorf("Status: got %q, want %q", loaded.Status, session.Status)
	}
	if loaded.ClaudeSessionID != session.ClaudeSessionID {
		t.Errorf("ClaudeSessionID: got %q, want %q", loaded.ClaudeSessionID, session.ClaudeSessionID)
	}
	if loaded.TmuxWindowName != session.TmuxWindowName {
		t.Errorf("TmuxWindowName: got %q, want %q", loaded.TmuxWindowName, session.TmuxWindowName)
	}
	if loaded.TmuxPaneID != session.TmuxPaneID {
		t.Errorf("TmuxPaneID: got %q, want %q", loaded.TmuxPaneID, session.TmuxPaneID)
	}
}

func TestStore_Load_NotFound(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	_, err = store.Load("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent ID")
	}
}

func TestStore_LoadAll_Multiple(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	for _, id := range []string{"a", "b", "c"} {
		s := &Session{ID: id, Name: "session-" + id, Status: StatusStopped}
		if err := store.Save(s); err != nil {
			t.Fatalf("Save(%s): %v", id, err)
		}
	}

	sessions, err := store.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("LoadAll: got %d, want 3", len(sessions))
	}

	ids := make(map[string]bool)
	for _, s := range sessions {
		ids[s.ID] = true
	}
	for _, id := range []string{"a", "b", "c"} {
		if !ids[id] {
			t.Errorf("session %q not found in LoadAll result", id)
		}
	}
}

func TestStore_LoadAll_SkipsInvalidFiles(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Valid session
	s := &Session{ID: "valid", Name: "valid-session", Status: StatusStopped}
	if err := store.Save(s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Invalid JSON file
	if err := os.WriteFile(filepath.Join(dir, "invalid.json"), []byte("{bad json"), 0644); err != nil {
		t.Fatalf("write invalid file: %v", err)
	}

	// Non-JSON file (should be skipped)
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("write txt file: %v", err)
	}

	sessions, err := store.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("LoadAll: got %d, want 1", len(sessions))
	}
	if sessions[0].ID != "valid" {
		t.Errorf("session ID: got %q, want %q", sessions[0].ID, "valid")
	}
}

func TestStore_Delete_Existing(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	s := &Session{ID: "delete-me", Status: StatusStopped}
	if err := store.Save(s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := store.Delete("delete-me"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = store.Load("delete-me")
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestStore_Delete_NonExistent(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	if err := store.Delete("does-not-exist"); err != nil {
		t.Fatalf("Delete non-existent: expected nil, got %v", err)
	}
}

func TestStore_RuntimeFieldsNotPersisted(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	s := &Session{
		ID:             "runtime-test",
		Status:         StatusRunning,
		CurrentWorkDir: "/runtime/dir",
		CurrentBranch:  "feature-x",
		IsGitRepo:      true,
		SSHAuthSock:    "/tmp/ssh-agent.sock",
		LastOutputTime: time.Now(),
		StartedAt:      time.Now(),
	}
	if err := store.Save(s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Read raw JSON to verify runtime fields are absent
	data, err := os.ReadFile(filepath.Join(dir, "runtime-test.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	runtimeFields := []string{
		"current_branch", "is_git_repo",
		"ssh_auth_sock", "last_output_time", "started_at",
	}
	for _, field := range runtimeFields {
		if _, found := raw[field]; found {
			t.Errorf("runtime field %q found in persisted JSON", field)
		}
	}
}
