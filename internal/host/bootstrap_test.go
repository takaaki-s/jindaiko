package host

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/takaaki-s/honjin/internal/config"
)

func TestIsNotInstalled(t *testing.T) {
	// Create a real *exec.ExitError with exit code 127
	_, exitErr127 := exec.Command("sh", "-c", "exit 127").Output()

	// Create a real *exec.ExitError with a different exit code
	_, exitErr1 := exec.Command("sh", "-c", "exit 1").Output()

	tests := []struct {
		name   string
		output string
		err    error
		want   bool
	}{
		{
			name:   "command not found",
			output: "jin: command not found",
			err:    nil,
			want:   true,
		},
		{
			name:   "not found",
			output: "jin: not found",
			err:    nil,
			want:   true,
		},
		{
			name:   "no such file or directory case insensitive",
			output: "jin: No such file or directory",
			err:    nil,
			want:   true,
		},
		{
			name:   "other error without jin mention",
			output: "some other error",
			err:    nil,
			want:   false,
		},
		{
			name:   "not found without jin",
			output: "not found",
			err:    nil,
			want:   false,
		},
		{
			name:   "multi-line with jin error on second line",
			output: "connecting to host...\njin: command not found",
			err:    nil,
			want:   true,
		},
		{
			name:   "nil error with no matching output",
			output: "",
			err:    nil,
			want:   false,
		},
		{
			name:   "exit code 127",
			output: "some output",
			err:    exitErr127,
			want:   true,
		},
		{
			name:   "exit code 1 without jin in output",
			output: "some output",
			err:    exitErr1,
			want:   false,
		},
		{
			name:   "generic error type not ExitError",
			output: "something",
			err:    fmt.Errorf("generic error"),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNotInstalled(tt.output, tt.err)
			if got != tt.want {
				t.Errorf("isNotInstalled(%q, %v) = %v, want %v", tt.output, tt.err, got, tt.want)
			}
		})
	}
}

func TestStartSlaveCommand(t *testing.T) {
	t.Run("ssh with default socket path", func(t *testing.T) {
		hc := config.HostConfig{
			Type: "ssh",
			Host: "myhost",
		}
		cmd := StartSlaveCommand(hc)
		if cmd == nil {
			t.Fatal("expected non-nil command")
		}

		// cmd.Args[0] is the program name itself
		wantArgs := []string{
			"ssh",
			"-o", "ControlMaster=no",
			"-o", "ClearAllForwardings=yes",
			"myhost",
			"jin daemon start --socket ~/.local/state/honjin/daemon.sock",
		}
		if len(cmd.Args) != len(wantArgs) {
			t.Fatalf("args length = %d, want %d\ngot:  %v\nwant: %v", len(cmd.Args), len(wantArgs), cmd.Args, wantArgs)
		}
		for i, arg := range wantArgs {
			if cmd.Args[i] != arg {
				t.Errorf("Args[%d] = %q, want %q", i, cmd.Args[i], arg)
			}
		}
	})

	t.Run("ssh with custom socket path", func(t *testing.T) {
		hc := config.HostConfig{
			Type:       "ssh",
			Host:       "myhost",
			SocketPath: "/tmp/custom.sock",
		}
		cmd := StartSlaveCommand(hc)
		if cmd == nil {
			t.Fatal("expected non-nil command")
		}

		wantRemoteCmd := "jin daemon start --socket /tmp/custom.sock"
		// The remote command is the last element of Args
		lastArg := cmd.Args[len(cmd.Args)-1]
		if lastArg != wantRemoteCmd {
			t.Errorf("remote command = %q, want %q", lastArg, wantRemoteCmd)
		}
	})

	t.Run("ssh with SSHOpts", func(t *testing.T) {
		hc := config.HostConfig{
			Type:    "ssh",
			Host:    "myhost",
			SSHOpts: []string{"-p", "2222", "-i", "/path/to/key"},
		}
		cmd := StartSlaveCommand(hc)
		if cmd == nil {
			t.Fatal("expected non-nil command")
		}

		wantArgs := []string{
			"ssh",
			"-o", "ControlMaster=no",
			"-o", "ClearAllForwardings=yes",
			"-p", "2222", "-i", "/path/to/key",
			"myhost",
			"jin daemon start --socket ~/.local/state/honjin/daemon.sock",
		}
		if len(cmd.Args) != len(wantArgs) {
			t.Fatalf("args length = %d, want %d\ngot:  %v\nwant: %v", len(cmd.Args), len(wantArgs), cmd.Args, wantArgs)
		}
		for i, arg := range wantArgs {
			if cmd.Args[i] != arg {
				t.Errorf("Args[%d] = %q, want %q", i, cmd.Args[i], arg)
			}
		}
	})

	t.Run("docker type", func(t *testing.T) {
		hc := config.HostConfig{
			Type:      "docker",
			Container: "my-container",
		}
		cmd := StartSlaveCommand(hc)
		if cmd == nil {
			t.Fatal("expected non-nil command")
		}

		wantArgs := []string{
			"docker", "exec", "my-container", "sh", "-c",
			"jin daemon start --socket ~/.local/state/honjin/daemon.sock",
		}
		if len(cmd.Args) != len(wantArgs) {
			t.Fatalf("args length = %d, want %d\ngot:  %v\nwant: %v", len(cmd.Args), len(wantArgs), cmd.Args, wantArgs)
		}
		for i, arg := range wantArgs {
			if cmd.Args[i] != arg {
				t.Errorf("Args[%d] = %q, want %q", i, cmd.Args[i], arg)
			}
		}
	})

	t.Run("unknown type returns nil", func(t *testing.T) {
		hc := config.HostConfig{
			Type: "unknown",
		}
		cmd := StartSlaveCommand(hc)
		if cmd != nil {
			t.Errorf("expected nil for unknown type, got %v", cmd)
		}
	})

	t.Run("ssh with BootstrapOptions", func(t *testing.T) {
		hc := config.HostConfig{Type: "ssh", Host: "myhost"}
		opts := BootstrapOptions{
			PeerSocketPath: "/tmp/jin-peers/mac/daemon.sock",
			PeerHostID:     "mac",
		}
		cmd := StartSlaveCommand(hc, opts)
		if cmd == nil {
			t.Fatal("expected non-nil command")
		}
		lastArg := cmd.Args[len(cmd.Args)-1]
		want := "jin daemon start --socket ~/.local/state/honjin/daemon.sock --peer-socket /tmp/jin-peers/mac/daemon.sock --peer-id mac"
		if lastArg != want {
			t.Errorf("remote command = %q, want %q", lastArg, want)
		}
	})

	t.Run("ssh with custom jin_path", func(t *testing.T) {
		hc := config.HostConfig{
			Type:    "ssh",
			Host:    "myhost",
			JinPath: "/home/user/.local/bin/jin",
		}
		cmd := StartSlaveCommand(hc)
		if cmd == nil {
			t.Fatal("expected non-nil command")
		}
		lastArg := cmd.Args[len(cmd.Args)-1]
		want := "/home/user/.local/bin/jin daemon start --socket ~/.local/state/honjin/daemon.sock"
		if lastArg != want {
			t.Errorf("remote command = %q, want %q", lastArg, want)
		}
	})

	t.Run("docker with custom jin_path", func(t *testing.T) {
		hc := config.HostConfig{
			Type:      "docker",
			Container: "my-container",
			JinPath:   "/usr/local/bin/jin",
		}
		cmd := StartSlaveCommand(hc)
		if cmd == nil {
			t.Fatal("expected non-nil command")
		}
		lastArg := cmd.Args[len(cmd.Args)-1]
		want := "/usr/local/bin/jin daemon start --socket ~/.local/state/honjin/daemon.sock"
		if lastArg != want {
			t.Errorf("remote command = %q, want %q", lastArg, want)
		}
	})
}

func TestValidateIdentifier(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"ec2", true},
		{"my-host", true},
		{"host_1", true},
		{"Mac", true},
		{"", false},
		{"host;cmd", false},
		{"host name", false},
		{"../etc", false},
	}
	for _, tt := range tests {
		err := ValidateIdentifier(tt.input)
		if tt.valid && err != nil {
			t.Errorf("ValidateIdentifier(%q) = %v, want nil", tt.input, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("ValidateIdentifier(%q) = nil, want error", tt.input)
		}
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"/tmp/jin-peers/mac/daemon.sock", true},
		{"~/.local/state/honjin/daemon.sock", true},
		{"/a/b/c", true},
		{"", false},
		{"/tmp/foo;rm -rf /", false},
		{"/tmp/foo bar", false},
		{"/tmp/$HOME", false},
	}
	for _, tt := range tests {
		err := validatePath(tt.input)
		if tt.valid && err != nil {
			t.Errorf("validatePath(%q) = %v, want nil", tt.input, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("validatePath(%q) = nil, want error", tt.input)
		}
	}
}
