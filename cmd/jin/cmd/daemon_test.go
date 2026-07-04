package cmd

import (
	"bytes"
	"encoding/json"
	"testing"
)

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
