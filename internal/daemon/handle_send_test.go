package daemon

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Cover the validation branches of handleSend without spinning a full Server,
// matching the style of handle_pane_test.go. Each case returns before touching
// s.manager, so the zero-value Server{} is safe.

func TestHandleSend_EmptyPrompt(t *testing.T) {
	s := &Server{}
	data, _ := json.Marshal(SendRequest{ID: "sess-1", Prompt: ""})
	resp := s.handleSend(data)

	if resp.Success {
		t.Fatal("expected Success=false")
	}
	if !strings.Contains(resp.Error, "prompt is required") {
		t.Errorf("Error = %q, want to contain 'prompt is required'", resp.Error)
	}
}

func TestHandleSend_WhitespaceOnlyPrompt(t *testing.T) {
	// Whitespace-only prompts must be rejected: Manager.SendPrompt's verify
	// treats them as trivially accepted (nothing meaningful to search for
	// in the pane), so letting one through would send an unverified Enter.
	cases := []string{
		" ",
		"\n",
		"\t",
		"  \n\t  ",
	}
	for _, prompt := range cases {
		t.Run(fmt.Sprintf("%q", prompt), func(t *testing.T) {
			s := &Server{}
			data, _ := json.Marshal(SendRequest{ID: "sess-1", Prompt: prompt})
			resp := s.handleSend(data)

			if resp.Success {
				t.Fatalf("expected Success=false for prompt=%q", prompt)
			}
			if !strings.Contains(resp.Error, "prompt is required") {
				t.Errorf("Error = %q, want to contain 'prompt is required'", resp.Error)
			}
		})
	}
}

func TestHandleSend_InvalidJSON(t *testing.T) {
	s := &Server{}
	resp := s.handleSend(json.RawMessage(`{`))

	if resp.Success {
		t.Fatal("expected Success=false")
	}
}
