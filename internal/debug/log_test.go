package debug

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewLogger_Disabled(t *testing.T) {
	// When CCVALET_DEBUG is not "1", the logger should be a no-op.
	origDebug := os.Getenv("CCVALET_DEBUG")
	os.Setenv("CCVALET_DEBUG", "0")
	defer os.Setenv("CCVALET_DEBUG", origDebug)

	// Reset enabled for this test
	origEnabled := enabled
	enabled = false
	defer func() { enabled = origEnabled }()

	dir := t.TempDir()
	filename := "test-disabled.log"

	// Override HOME so NewLogger resolves to our temp dir
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	// Create .ccvalet directory
	if err := os.MkdirAll(filepath.Join(dir, ".ccvalet"), 0755); err != nil {
		t.Fatalf("failed to create .ccvalet dir: %v", err)
	}

	log := NewLogger(filename)
	log("this message should not appear")

	logPath := filepath.Join(dir, ".ccvalet", filename)
	if _, err := os.Stat(logPath); err == nil {
		t.Error("logger created a file even though debug is disabled")
	}
}

func TestNewLogger_Enabled(t *testing.T) {
	origEnabled := enabled
	enabled = true
	defer func() { enabled = origEnabled }()

	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	if err := os.MkdirAll(filepath.Join(dir, ".ccvalet"), 0755); err != nil {
		t.Fatalf("failed to create .ccvalet dir: %v", err)
	}

	log := NewLogger("test-enabled.log")
	log("hello %s %d", "world", 42)

	logPath := filepath.Join(dir, ".ccvalet", "test-enabled.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "hello world 42") {
		t.Errorf("log file content %q does not contain expected message", content)
	}
	// Verify timestamp format [HH:MM:SS.mmm] is present
	if !strings.Contains(content, "[") || !strings.Contains(content, "]") {
		t.Errorf("log file content %q does not contain timestamp brackets", content)
	}
}

func TestNewLogger_NoopWhenDisabled(t *testing.T) {
	origEnabled := enabled
	enabled = false
	defer func() { enabled = origEnabled }()

	log := NewLogger("noop.log")

	// Should not panic
	log("this should be a no-op")
}
