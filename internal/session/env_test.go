package session

import (
	"strings"
	"testing"
)

func TestBuildEnvString_Empty(t *testing.T) {
	got := buildEnvString(nil)
	if got != "" {
		t.Errorf("buildEnvString(nil) = %q, want empty string", got)
	}

	got = buildEnvString(map[string]string{})
	if got != "" {
		t.Errorf("buildEnvString(empty) = %q, want empty string", got)
	}
}

func TestBuildEnvString_SingleVar(t *testing.T) {
	got := buildEnvString(map[string]string{"MY_VAR": "hello"})
	want := "MY_VAR='hello'"
	if got != want {
		t.Errorf("buildEnvString = %q, want %q", got, want)
	}
}

func TestBuildEnvString_MultipleVarsSorted(t *testing.T) {
	env := map[string]string{
		"ZEBRA":  "z",
		"ALPHA":  "a",
		"MIDDLE": "m",
	}
	got := buildEnvString(env)
	want := "ALPHA='a' MIDDLE='m' ZEBRA='z'"
	if got != want {
		t.Errorf("buildEnvString = %q, want %q", got, want)
	}
}

func TestBuildEnvString_ShellEscaping(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "value with single quote",
			value: "it's here",
			want:  "KEY='it'\\''s here'",
		},
		{
			name:  "value with spaces",
			value: "hello world",
			want:  "KEY='hello world'",
		},
		{
			name:  "value with double quotes",
			value: `say "hi"`,
			want:  `KEY='say "hi"'`,
		},
		{
			name:  "value with special chars",
			value: "a$b`c\\d",
			want:  "KEY='a$b`c\\d'",
		},
		{
			name:  "empty value",
			value: "",
			want:  "KEY=''",
		},
		{
			name:  "value with multiple single quotes",
			value: "a'b'c",
			want:  "KEY='a'\\''b'\\''c'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildEnvString(map[string]string{"KEY": tt.value})
			if got != tt.want {
				t.Errorf("buildEnvString = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildEnvString_InvalidKeysSkipped(t *testing.T) {
	env := map[string]string{
		"VALID_KEY":    "ok",
		"123INVALID":   "bad",
		"ALSO-INVALID": "bad",
		"KEY WITH SPC": "bad",
		"":             "bad",
		"OK_2":         "ok",
	}
	got := buildEnvString(env)

	// Only VALID_KEY and OK_2 should be present (sorted)
	want := "OK_2='ok' VALID_KEY='ok'"
	if got != want {
		t.Errorf("buildEnvString = %q, want %q", got, want)
	}

	// Make sure invalid keys are not present
	for _, bad := range []string{"123INVALID", "ALSO-INVALID", "KEY WITH SPC"} {
		if strings.Contains(got, bad) {
			t.Errorf("buildEnvString output contains invalid key %q", bad)
		}
	}
}

func TestBuildEnvString_ValidKeyPatterns(t *testing.T) {
	// These should all be accepted
	validKeys := map[string]string{
		"A":          "v",
		"_PRIVATE":   "v",
		"MY_VAR_123": "v",
		"lowercase":  "v",
		"MixedCase":  "v",
		"_":          "v",
	}
	got := buildEnvString(validKeys)

	// All 6 keys should be present
	parts := strings.Split(got, " ")
	if len(parts) != 6 {
		t.Errorf("expected 6 env vars, got %d: %q", len(parts), got)
	}
}
