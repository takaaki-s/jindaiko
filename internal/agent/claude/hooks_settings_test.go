package claude

import (
	"encoding/json"
	"os"
	"testing"
)

func TestEnsureHooksSettingsFile_NewHooks(t *testing.T) {
	dir := t.TempDir()
	path, err := EnsureHooksSettingsFile(dir, "/usr/local/bin/jin")
	if err != nil {
		t.Fatalf("EnsureHooksSettingsFile failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var settings hooksSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	requiredHooks := []string{"UserPromptSubmit", "Stop", "StopFailure", "PostToolUse", "CwdChanged", "SessionStart", "SessionEnd", "Notification"}
	for _, hook := range requiredHooks {
		if _, ok := settings.Hooks[hook]; !ok {
			t.Errorf("hooks-settings.json missing hook: %s", hook)
		}
	}
}

// EnsureHooksSettingsFile is called from Agent.Setup under sync.Once, but the
// helper itself must be safe to invoke repeatedly (Setup runs per-session-start
// even though the write itself is guarded). Verify the file content is stable
// across a second call.
func TestEnsureHooksSettingsFile_Idempotent(t *testing.T) {
	dir := t.TempDir()

	path1, err := EnsureHooksSettingsFile(dir, "/usr/local/bin/jin")
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	first, err := os.ReadFile(path1)
	if err != nil {
		t.Fatalf("first read failed: %v", err)
	}

	path2, err := EnsureHooksSettingsFile(dir, "/usr/local/bin/jin")
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	second, err := os.ReadFile(path2)
	if err != nil {
		t.Fatalf("second read failed: %v", err)
	}

	if path1 != path2 {
		t.Errorf("path differed across calls: %q vs %q", path1, path2)
	}
	if string(first) != string(second) {
		t.Errorf("content differed across calls:\nfirst=%s\nsecond=%s", first, second)
	}
}
