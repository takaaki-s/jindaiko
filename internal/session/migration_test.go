package session

import (
	"encoding/json"
	"testing"
)

func TestMigrateSessionJSON(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantChanged    bool
		wantErr        bool
		wantDesc       string
		wantLocked     bool
		wantNameAbsent bool
	}{
		{
			name:           "legacy schema with non-empty name migrates to description locked",
			input:          `{"id":"abc","name":"my-session","work_dir":"/tmp/x"}`,
			wantChanged:    true,
			wantDesc:       "my-session",
			wantLocked:     true,
			wantNameAbsent: true,
		},
		{
			name:           "legacy schema with empty name only drops the field",
			input:          `{"id":"abc","name":"","work_dir":"/tmp/x"}`,
			wantChanged:    true,
			wantDesc:       "",
			wantLocked:     false,
			wantNameAbsent: true,
		},
		{
			name:        "new schema is untouched",
			input:       `{"id":"abc","description":"my-session","description_locked":true,"work_dir":"/tmp/x"}`,
			wantChanged: false,
			wantDesc:    "my-session",
			wantLocked:  true,
		},
		{
			name:           "existing description takes precedence over legacy name",
			input:          `{"id":"abc","name":"old","description":"new","work_dir":"/tmp/x"}`,
			wantChanged:    true,
			wantDesc:       "new",
			wantLocked:     false,
			wantNameAbsent: true,
		},
		{
			name:    "broken JSON returns an error",
			input:   `{not json`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, changed, err := migrateSessionJSON([]byte(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if changed != tc.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tc.wantChanged)
			}

			var got map[string]any
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatalf("output is not valid JSON: %v", err)
			}

			if desc, _ := got["description"].(string); desc != tc.wantDesc {
				t.Errorf("description = %q, want %q", desc, tc.wantDesc)
			}
			locked, _ := got["description_locked"].(bool)
			if locked != tc.wantLocked {
				t.Errorf("description_locked = %v, want %v", locked, tc.wantLocked)
			}
			if tc.wantNameAbsent {
				if _, ok := got["name"]; ok {
					t.Errorf("name field should be removed, still present: %v", got["name"])
				}
			}
		})
	}
}

func TestMigrateSessionJSON_Idempotent(t *testing.T) {
	input := []byte(`{"id":"abc","name":"legacy","work_dir":"/tmp/x"}`)

	first, changed1, err := migrateSessionJSON(input)
	if err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if !changed1 {
		t.Fatal("first migrate should report changed=true")
	}

	second, changed2, err := migrateSessionJSON(first)
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if changed2 {
		t.Fatal("second migrate should report changed=false (idempotent)")
	}
	if string(second) != string(first) {
		t.Errorf("second migrate mutated output:\nfirst=%s\nsecond=%s", first, second)
	}
}
