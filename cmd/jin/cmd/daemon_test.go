package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/takaaki-s/jind-ai/internal/paths"
)

func TestGetSocketPath(t *testing.T) {
	t.Run("flag takes precedence", func(t *testing.T) {
		orig := socketPathFlag
		socketPathFlag = "/tmp/flag.sock"
		defer func() { socketPathFlag = orig }()
		t.Setenv("JIN_SOCKET", "/tmp/env.sock")

		if got := getSocketPath(); got != "/tmp/flag.sock" {
			t.Errorf("getSocketPath() = %q, want %q", got, "/tmp/flag.sock")
		}
	})

	t.Run("env used when flag unset", func(t *testing.T) {
		orig := socketPathFlag
		socketPathFlag = ""
		defer func() { socketPathFlag = orig }()
		t.Setenv("JIN_SOCKET", "/tmp/env.sock")

		if got := getSocketPath(); got != "/tmp/env.sock" {
			t.Errorf("getSocketPath() = %q, want %q", got, "/tmp/env.sock")
		}
	})

	t.Run("falls back to paths.Socket when neither set", func(t *testing.T) {
		orig := socketPathFlag
		socketPathFlag = ""
		defer func() { socketPathFlag = orig }()
		t.Setenv("JIN_SOCKET", "")

		if got := getSocketPath(); got != paths.Socket() {
			t.Errorf("getSocketPath() = %q, want %q", got, paths.Socket())
		}
	})
}

func TestRenderDaemonStatusJSON(t *testing.T) {
	t.Run("running", func(t *testing.T) {
		result := daemonStatusResult{Running: true}
		var buf bytes.Buffer
		if err := renderDaemonStatusJSON(&buf, result); err != nil {
			t.Fatalf("renderDaemonStatusJSON() error = %v", err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
		}
		if parsed["running"] != true {
			t.Errorf("expected running=true, got %v", parsed["running"])
		}
	})

	t.Run("not running", func(t *testing.T) {
		result := daemonStatusResult{Running: false}
		var buf bytes.Buffer
		if err := renderDaemonStatusJSON(&buf, result); err != nil {
			t.Fatalf("renderDaemonStatusJSON() error = %v", err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
		}
		if parsed["running"] != false {
			t.Errorf("expected running=false, got %v", parsed["running"])
		}
	})
}

func TestRenderDaemonRestartJSON(t *testing.T) {
	t.Run("stopped and started", func(t *testing.T) {
		result := daemonRestartResult{Stopped: true, Started: true, PID: 4242}
		var buf bytes.Buffer
		if err := renderDaemonRestartJSON(&buf, result); err != nil {
			t.Fatalf("renderDaemonRestartJSON() error = %v", err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
		}
		if parsed["stopped"] != true {
			t.Errorf("expected stopped=true, got %v", parsed["stopped"])
		}
		if parsed["started"] != true {
			t.Errorf("expected started=true, got %v", parsed["started"])
		}
		// json numbers decode to float64
		if pid, ok := parsed["pid"].(float64); !ok || int(pid) != 4242 {
			t.Errorf("expected pid=4242, got %v", parsed["pid"])
		}
	})

	t.Run("cold start omits pid=0 when nothing started", func(t *testing.T) {
		result := daemonRestartResult{Stopped: false, Started: false, PID: 0}
		var buf bytes.Buffer
		if err := renderDaemonRestartJSON(&buf, result); err != nil {
			t.Fatalf("renderDaemonRestartJSON() error = %v", err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
		}
		if parsed["stopped"] != false {
			t.Errorf("expected stopped=false, got %v", parsed["stopped"])
		}
		if parsed["started"] != false {
			t.Errorf("expected started=false, got %v", parsed["started"])
		}
		if _, present := parsed["pid"]; present {
			t.Errorf("expected pid field to be omitted when zero, got %v", parsed["pid"])
		}
	})

	t.Run("no daemon running, fresh start", func(t *testing.T) {
		result := daemonRestartResult{Stopped: false, Started: true, PID: 99}
		var buf bytes.Buffer
		if err := renderDaemonRestartJSON(&buf, result); err != nil {
			t.Fatalf("renderDaemonRestartJSON() error = %v", err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
		}
		if parsed["stopped"] != false {
			t.Errorf("expected stopped=false, got %v", parsed["stopped"])
		}
		if parsed["started"] != true {
			t.Errorf("expected started=true, got %v", parsed["started"])
		}
		if pid, ok := parsed["pid"].(float64); !ok || int(pid) != 99 {
			t.Errorf("expected pid=99, got %v", parsed["pid"])
		}
	})
}

func TestStopDaemonIfRunning_notRunning(t *testing.T) {
	// Point at a socket path guaranteed not to exist so IsRunning() returns false.
	orig := socketPathFlag
	socketPathFlag = "/tmp/jin-restart-test-does-not-exist.sock"
	defer func() { socketPathFlag = orig }()

	stopped, err := stopDaemonIfRunning()
	if err != nil {
		t.Fatalf("stopDaemonIfRunning() error = %v", err)
	}
	if stopped {
		t.Errorf("expected stopped=false when no daemon is listening, got true")
	}
}

func TestDaemonRestartCmd_wiredAsSubcommand(t *testing.T) {
	// Make sure the subcommand is actually attached — the whole point of this
	// change is that the error message pointing at 'jin daemon restart' resolves
	// to a real command.
	sub, _, err := daemonCmd.Find([]string{"restart"})
	if err != nil {
		t.Fatalf("daemonCmd.Find(restart) error = %v", err)
	}
	if sub == nil || sub.Name() != "restart" {
		t.Fatalf("expected daemon restart subcommand, got %v", sub)
	}
}
