package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureTrustState(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	workDir := t.TempDir()

	if err := EnsureTrustState(workDir); err != nil {
		t.Fatalf("EnsureTrustState failed: %v", err)
	}

	settingsPath := filepath.Join(fakeHome, ".claude", "settings.local.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("failed to read settings file: %v", err)
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("failed to unmarshal settings: %v", err)
	}

	absWorkDir, _ := filepath.Abs(workDir)
	projectSettings, exists := settings.Projects[absWorkDir]
	if !exists {
		t.Fatalf("project settings not found for %s, got keys: %v", absWorkDir, func() []string {
			keys := make([]string, 0, len(settings.Projects))
			for k := range settings.Projects {
				keys = append(keys, k)
			}
			return keys
		}())
	}
	if !projectSettings.HasTrustDialogAccepted {
		t.Error("HasTrustDialogAccepted = false, want true")
	}
}

func TestEnsureTrustState_Idempotent(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	workDir := t.TempDir()

	if err := EnsureTrustState(workDir); err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if err := EnsureTrustState(workDir); err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	settingsPath := filepath.Join(fakeHome, ".claude", "settings.local.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("failed to read settings file: %v", err)
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("failed to unmarshal settings: %v", err)
	}

	absWorkDir, _ := filepath.Abs(workDir)
	if len(settings.Projects) != 1 {
		t.Errorf("expected 1 project entry, got %d", len(settings.Projects))
	}
	if !settings.Projects[absWorkDir].HasTrustDialogAccepted {
		t.Error("HasTrustDialogAccepted = false, want true")
	}
}

func TestEnsureTrustState_MultipleProjects(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	workDir1 := t.TempDir()
	workDir2 := t.TempDir()

	if err := EnsureTrustState(workDir1); err != nil {
		t.Fatalf("first project failed: %v", err)
	}
	if err := EnsureTrustState(workDir2); err != nil {
		t.Fatalf("second project failed: %v", err)
	}

	settingsPath := filepath.Join(fakeHome, ".claude", "settings.local.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("failed to read settings file: %v", err)
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("failed to unmarshal settings: %v", err)
	}

	if len(settings.Projects) != 2 {
		t.Errorf("expected 2 project entries, got %d", len(settings.Projects))
	}

	absWorkDir1, _ := filepath.Abs(workDir1)
	absWorkDir2, _ := filepath.Abs(workDir2)

	if !settings.Projects[absWorkDir1].HasTrustDialogAccepted {
		t.Error("project 1 HasTrustDialogAccepted = false, want true")
	}
	if !settings.Projects[absWorkDir2].HasTrustDialogAccepted {
		t.Error("project 2 HasTrustDialogAccepted = false, want true")
	}
}
