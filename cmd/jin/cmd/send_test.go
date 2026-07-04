package cmd

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRenderSendResultJSON(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		result := sendResult{Success: true, Session: "my-session"}
		var buf bytes.Buffer
		if err := renderSendResultJSON(&buf, result); err != nil {
			t.Fatalf("renderSendResultJSON() error = %v", err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
		}
		if parsed["success"] != true {
			t.Errorf("expected success=true, got %v", parsed["success"])
		}
		if parsed["session"] != "my-session" {
			t.Errorf("expected session=%q, got %v", "my-session", parsed["session"])
		}
	})
}
