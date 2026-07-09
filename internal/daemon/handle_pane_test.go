package daemon

import (
	"encoding/json"
	"strings"
	"testing"
)

// These tests exercise the validation branches of the pane-* handlers without
// spinning a full Server, in the style of handle_new_test.go. Every case here
// returns before touching s.manager, so the zero-value Server{} is safe.

func TestHandlePanePopup_MissingID(t *testing.T) {
	s := &Server{}
	data, _ := json.Marshal(PanePopupRequest{Cmd: "echo hi"})
	resp := s.handlePanePopup(data)

	if resp.Success {
		t.Fatal("expected Success=false")
	}
	if !strings.Contains(resp.Error, "id is required") {
		t.Errorf("Error = %q, want to contain 'id is required'", resp.Error)
	}
}

func TestHandlePanePopup_MissingCmd(t *testing.T) {
	s := &Server{}
	data, _ := json.Marshal(PanePopupRequest{ID: "sess-1"})
	resp := s.handlePanePopup(data)

	if resp.Success {
		t.Fatal("expected Success=false")
	}
	if !strings.Contains(resp.Error, "cmd is required") {
		t.Errorf("Error = %q, want to contain 'cmd is required'", resp.Error)
	}
}

func TestHandlePanePopup_InvalidJSON(t *testing.T) {
	s := &Server{}
	resp := s.handlePanePopup(json.RawMessage(`{`))

	if resp.Success {
		t.Fatal("expected Success=false")
	}
}

func TestHandlePaneSplit_MissingID(t *testing.T) {
	s := &Server{}
	data, _ := json.Marshal(PaneSplitRequest{Cmd: "top"})
	resp := s.handlePaneSplit(data)

	if resp.Success {
		t.Fatal("expected Success=false")
	}
	if !strings.Contains(resp.Error, "id is required") {
		t.Errorf("Error = %q, want to contain 'id is required'", resp.Error)
	}
}

func TestHandlePaneSplit_InvalidJSON(t *testing.T) {
	s := &Server{}
	resp := s.handlePaneSplit(json.RawMessage(`{`))

	if resp.Success {
		t.Fatal("expected Success=false")
	}
}

func TestHandlePaneCapture_MissingID(t *testing.T) {
	s := &Server{}
	data, _ := json.Marshal(PaneCaptureRequest{})
	resp := s.handlePaneCapture(data)

	if resp.Success {
		t.Fatal("expected Success=false")
	}
	if !strings.Contains(resp.Error, "id is required") {
		t.Errorf("Error = %q, want to contain 'id is required'", resp.Error)
	}
}

func TestHandlePaneCapture_InvalidJSON(t *testing.T) {
	s := &Server{}
	resp := s.handlePaneCapture(json.RawMessage(`{`))

	if resp.Success {
		t.Fatal("expected Success=false")
	}
}

func TestHandlePaneSendKeys_MissingID(t *testing.T) {
	s := &Server{}
	data, _ := json.Marshal(PaneSendKeysRequest{Keys: "hello"})
	resp := s.handlePaneSendKeys(data)

	if resp.Success {
		t.Fatal("expected Success=false")
	}
	if !strings.Contains(resp.Error, "id is required") {
		t.Errorf("Error = %q, want to contain 'id is required'", resp.Error)
	}
}

func TestHandlePaneSendKeys_MissingKeys(t *testing.T) {
	s := &Server{}
	data, _ := json.Marshal(PaneSendKeysRequest{ID: "sess-1"})
	resp := s.handlePaneSendKeys(data)

	if resp.Success {
		t.Fatal("expected Success=false")
	}
	if !strings.Contains(resp.Error, "keys is required") {
		t.Errorf("Error = %q, want to contain 'keys is required'", resp.Error)
	}
}

func TestHandlePaneSendKeys_InvalidJSON(t *testing.T) {
	s := &Server{}
	resp := s.handlePaneSendKeys(json.RawMessage(`{`))

	if resp.Success {
		t.Fatal("expected Success=false")
	}
}
