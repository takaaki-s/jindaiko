package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsProcessRunning(t *testing.T) {
	tests := []struct {
		name string
		s    *Session
		want bool
	}{
		{
			name: "Stopped session is not running",
			s:    &Session{Status: StatusStopped},
			want: false,
		},
		{
			name: "Running session with TmuxWindowName is running",
			s:    &Session{Status: StatusRunning, TmuxWindowName: "jin_test-123"},
			want: true,
		},
		{
			name: "Thinking session without TmuxWindowName is not running",
			s:    &Session{Status: StatusThinking, TmuxWindowName: ""},
			want: false,
		},
		{
			name: "Idle session with TmuxWindowName is running",
			s:    &Session{Status: StatusIdle, TmuxWindowName: "jin_idle"},
			want: true,
		},
		{
			name: "Creating session without TmuxWindowName is not running",
			s:    &Session{Status: StatusCreating, TmuxWindowName: ""},
			want: false,
		},
		{
			name: "Permission session with TmuxWindowName is running",
			s:    &Session{Status: StatusPermission, TmuxWindowName: "jin_perm"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isProcessRunning(tt.s)
			if got != tt.want {
				t.Errorf("isProcessRunning() = %v, want %v (status=%q, tmux=%q)",
					got, tt.want, tt.s.Status, tt.s.TmuxWindowName)
			}
		})
	}
}

func TestExpandTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir() failed: %v", err)
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "bare tilde expands to home",
			input: "~",
			want:  home,
		},
		{
			name:  "tilde with subpath expands",
			input: "~/foo",
			want:  filepath.Join(home, "foo"),
		},
		{
			name:  "tilde with nested subpath expands",
			input: "~/foo/bar/baz",
			want:  filepath.Join(home, "foo", "bar", "baz"),
		},
		{
			name:  "absolute path unchanged",
			input: "/absolute/path",
			want:  "/absolute/path",
		},
		{
			name:  "relative path unchanged",
			input: "relative/path",
			want:  "relative/path",
		},
		{
			name:  "tilde in middle unchanged",
			input: "/home/~user",
			want:  "/home/~user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandTilde(tt.input)
			if got != tt.want {
				t.Errorf("expandTilde(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestWorkDirForShell(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "bare tilde becomes $HOME",
			input: "~",
			want:  "$HOME",
		},
		{
			name:  "tilde with subpath becomes $HOME/subpath",
			input: "~/foo",
			want:  "$HOME/foo",
		},
		{
			name:  "tilde with nested subpath",
			input: "~/foo/bar",
			want:  "$HOME/foo/bar",
		},
		{
			name:  "absolute path unchanged",
			input: "/absolute/path",
			want:  "/absolute/path",
		},
		{
			name:  "relative path unchanged",
			input: "relative/path",
			want:  "relative/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := workDirForShell(tt.input)
			if got != tt.want {
				t.Errorf("workDirForShell(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
