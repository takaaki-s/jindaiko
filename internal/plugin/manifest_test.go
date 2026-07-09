package plugin

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeManifest writes content as jin-plugin.yaml into a fresh temp dir and
// returns that dir, so tests can exercise LoadManifest end to end.
func writeManifest(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ManifestFilename), []byte(content), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return dir
}

func TestLoadManifest_Valid(t *testing.T) {
	dir := writeManifest(t, `
name: notifier
api_version: 1
on:
  - status_changed
  - status_changed:permission
run: ./notify.sh
build: go build -o notify .
timeout: 45s
`)

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.Name != "notifier" {
		t.Errorf("Name = %q, want notifier", m.Name)
	}
	if m.APIVersion != 1 {
		t.Errorf("APIVersion = %d, want 1", m.APIVersion)
	}
	if len(m.On) != 2 || m.On[0] != "status_changed" || m.On[1] != "status_changed:permission" {
		t.Errorf("On = %v, unexpected", m.On)
	}
	if m.Run != "./notify.sh" {
		t.Errorf("Run = %q, want ./notify.sh", m.Run)
	}
	if m.Build != "go build -o notify ." {
		t.Errorf("Build = %q, unexpected", m.Build)
	}
	if m.Timeout != 45*time.Second {
		t.Errorf("Timeout = %s, want 45s", m.Timeout)
	}
	if got := m.EffectiveTimeout(); got != 45*time.Second {
		t.Errorf("EffectiveTimeout = %s, want 45s", got)
	}
}

func TestLoadManifest_MissingFile(t *testing.T) {
	if _, err := LoadManifest(t.TempDir()); err == nil {
		t.Fatal("LoadManifest on empty dir: want error, got nil")
	}
}

func TestLoadManifest_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "name missing",
			content: `
api_version: 1
run: ./run.sh
`,
		},
		{
			name: "name grammar violation",
			content: `
name: Bad_Name
api_version: 1
run: ./run.sh
`,
		},
		{
			name: "api_version missing",
			content: `
name: notifier
run: ./run.sh
`,
		},
		{
			name: "run missing",
			content: `
name: notifier
api_version: 1
`,
		},
		{
			name: "on grammar violation",
			content: `
name: notifier
api_version: 1
run: ./run.sh
on:
  - file_changed
`,
		},
		{
			name: "on empty status",
			content: `
name: notifier
api_version: 1
run: ./run.sh
on:
  - status_changed:
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeManifest(t, tt.content)
			if _, err := LoadManifest(dir); err == nil {
				t.Errorf("LoadManifest: want error for %q, got nil", tt.name)
			}
		})
	}
}

func TestLoadManifest_TimeoutParseError(t *testing.T) {
	dir := writeManifest(t, `
name: notifier
api_version: 1
run: ./run.sh
timeout: "not-a-duration"
`)
	if _, err := LoadManifest(dir); err == nil {
		t.Fatal("LoadManifest with bad timeout: want error, got nil")
	}
}

func TestEffectiveTimeout_DefaultWhenUnset(t *testing.T) {
	dir := writeManifest(t, `
name: notifier
api_version: 1
run: ./run.sh
`)
	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.Timeout != 0 {
		t.Errorf("Timeout = %s, want 0 (unset)", m.Timeout)
	}
	if got := m.EffectiveTimeout(); got != DefaultTimeout {
		t.Errorf("EffectiveTimeout = %s, want %s", got, DefaultTimeout)
	}
}

func TestValidateMatcher(t *testing.T) {
	valid := []string{"status_changed", "status_changed:idle", "status_changed:permission"}
	for _, m := range valid {
		if err := ValidateMatcher(m); err != nil {
			t.Errorf("ValidateMatcher(%q) = %v, want nil", m, err)
		}
	}

	invalid := []string{"", "status_changed:", "file_changed"}
	for _, m := range invalid {
		if err := ValidateMatcher(m); err == nil {
			t.Errorf("ValidateMatcher(%q) = nil, want error", m)
		}
	}
}

func TestMatcherMatches(t *testing.T) {
	tests := []struct {
		matcher string
		event   string
		status  string
		want    bool
	}{
		{"status_changed", "status_changed", "idle", true},
		{"status_changed", "status_changed", "permission", true},
		{"status_changed:permission", "status_changed", "permission", true},
		{"status_changed:permission", "status_changed", "idle", false},
		{"status_changed:idle", "status_changed", "idle", true},
		{"status_changed", "other_event", "idle", false},
	}
	for _, tt := range tests {
		if got := MatcherMatches(tt.matcher, tt.event, tt.status); got != tt.want {
			t.Errorf("MatcherMatches(%q, %q, %q) = %v, want %v",
				tt.matcher, tt.event, tt.status, got, tt.want)
		}
	}
}
