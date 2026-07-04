package daemon

import (
	"encoding/json"
	"testing"

	"github.com/takaaki-s/honjin/internal/host"
)

func TestRequest_VisitedJSON_RoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		req     Request
		wantLen int
	}{
		{
			name:    "with visited",
			req:     Request{Action: "list", Visited: []string{"mac", "ec2"}},
			wantLen: 2,
		},
		{
			name:    "empty visited omitted",
			req:     Request{Action: "list"},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.req)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			var got Request
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			if len(got.Visited) != tt.wantLen {
				t.Errorf("Visited len = %d, want %d", len(got.Visited), tt.wantLen)
			}

			if tt.wantLen > 0 {
				if got.Visited[0] != "mac" || got.Visited[1] != "ec2" {
					t.Errorf("Visited = %v, want [mac ec2]", got.Visited)
				}
			}
		})
	}
}

func TestRequest_VisitedJSON_OmitEmpty(t *testing.T) {
	req := Request{Action: "list"}
	data, _ := json.Marshal(req)

	var m map[string]json.RawMessage
	_ = json.Unmarshal(data, &m)

	if _, ok := m["visited"]; ok {
		t.Error("visited should be omitted when empty")
	}
}

func TestRequest_VisitedJSON_BackwardCompat(t *testing.T) {
	// Old format without visited field should still work
	old := `{"action":"list","data":null}`
	var req Request
	if err := json.Unmarshal([]byte(old), &req); err != nil {
		t.Fatalf("Unmarshal old format: %v", err)
	}
	if req.Visited != nil {
		t.Errorf("Visited should be nil for old format, got %v", req.Visited)
	}
}

func TestForwardToHost_VisitedCheck(t *testing.T) {
	// No hostRegistry → should error with "not initialized"
	s := &Server{hostID: "mac"}
	req := Request{Action: "get", Visited: []string{"mac"}}
	resp := s.forwardToHost("ec2", req)
	if resp.Success {
		t.Error("should fail when no host registry")
	}
	if resp.Error != "host registry not initialized" {
		t.Errorf("error = %q, want 'host registry not initialized'", resp.Error)
	}

	// With registry, target already in visited → routing loop
	s.hostRegistry = host.NewRegistry(nil)
	req2 := Request{Action: "get", Visited: []string{"ec2"}}
	resp2 := s.forwardToHost("ec2", req2)
	if resp2.Success {
		t.Error("should fail when target is in visited")
	}
	if resp2.Error != "routing loop detected" {
		t.Errorf("error = %q, want 'routing loop detected'", resp2.Error)
	}

	// With registry, target not visited but unknown host
	req3 := Request{Action: "get", Visited: []string{"mac"}}
	resp3 := s.forwardToHost("unknown", req3)
	if resp3.Success {
		t.Error("should fail for unknown host")
	}
	if resp3.Error != "unknown host: unknown" {
		t.Errorf("error = %q, want 'unknown host: unknown'", resp3.Error)
	}
}
