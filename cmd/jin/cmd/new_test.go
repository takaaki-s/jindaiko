package cmd

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/takaaki-s/honjin/internal/session"
)

func TestRenderNewSessionJSON(t *testing.T) {
	t.Run("outputs session info as JSON", func(t *testing.T) {
		info := &session.Info{
			ID:        "abc-123",
			Name:      "my-session",
			WorkDir:   "/home/user/project",
			Status:    session.StatusCreating,
			CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		}
		var buf bytes.Buffer
		if err := renderNewSessionJSON(&buf, info); err != nil {
			t.Fatalf("renderNewSessionJSON() error = %v", err)
		}
		var parsed session.Info
		if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
		}
		if parsed.ID != "abc-123" {
			t.Errorf("expected id %q, got %q", "abc-123", parsed.ID)
		}
		if parsed.Name != "my-session" {
			t.Errorf("expected name %q, got %q", "my-session", parsed.Name)
		}
		if parsed.Status != session.StatusCreating {
			t.Errorf("expected status %q, got %q", session.StatusCreating, parsed.Status)
		}
	})
}
