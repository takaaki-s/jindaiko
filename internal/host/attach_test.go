package host

import (
	"strings"
	"testing"

	"github.com/takaaki-s/honjin/internal/config"
)

func TestSSHControlPath(t *testing.T) {
	got := SSHControlPath("ec2")
	want := "/tmp/jin-ssh-ctrl-ec2"
	if got != want {
		t.Errorf("SSHControlPath(%q) = %q, want %q", "ec2", got, want)
	}
}

func TestSSHControlPath_DifferentHost(t *testing.T) {
	got := SSHControlPath("docker-dev")
	want := "/tmp/jin-ssh-ctrl-docker-dev"
	if got != want {
		t.Errorf("SSHControlPath(%q) = %q, want %q", "docker-dev", got, want)
	}
}

func TestAttachCommandString_SSH(t *testing.T) {
	cfg := config.HostConfig{
		ID:      "ec2",
		Type:    "ssh",
		Host:    "ec2-host",
		SSHOpts: []string{"-p", "2222"},
	}
	target := "my-session"
	got := AttachCommandString(cfg, target)

	// Verify it starts with ssh command
	if !strings.HasPrefix(got, "ssh ") {
		t.Errorf("SSH command should start with 'ssh ', got %q", got)
	}

	// Verify ControlMaster=no is present
	if !strings.Contains(got, "ControlMaster=no") {
		t.Errorf("SSH command should contain ControlMaster=no, got %q", got)
	}

	// Verify ControlPath references the right socket
	expectedCtrlPath := SSHControlPath("ec2")
	if !strings.Contains(got, "ControlPath="+expectedCtrlPath) {
		t.Errorf("SSH command should contain ControlPath=%s, got %q", expectedCtrlPath, got)
	}

	// Verify ClearAllForwardings=yes
	if !strings.Contains(got, "ClearAllForwardings=yes") {
		t.Errorf("SSH command should contain ClearAllForwardings=yes, got %q", got)
	}

	// Verify SSH opts are included
	if !strings.Contains(got, "-p 2222") {
		t.Errorf("SSH command should contain SSHOpts '-p 2222', got %q", got)
	}

	// Verify -t and host
	if !strings.Contains(got, "-t ec2-host") {
		t.Errorf("SSH command should contain '-t ec2-host', got %q", got)
	}

	// Verify remote tmux command is quoted
	expectedRemote := "tmux -L jin attach -t my-session"
	if !strings.Contains(got, "'"+expectedRemote+"'") {
		t.Errorf("SSH command should contain quoted remote command '%s', got %q", expectedRemote, got)
	}
}

func TestAttachCommandString_Docker(t *testing.T) {
	cfg := config.HostConfig{
		ID:        "dev",
		Type:      "docker",
		Container: "my-container",
	}
	target := "my-session"
	got := AttachCommandString(cfg, target)

	want := "docker exec -it my-container tmux -L jin attach -t my-session"
	if got != want {
		t.Errorf("AttachCommandString(docker) = %q, want %q", got, want)
	}
}

func TestAttachCommandString_Local(t *testing.T) {
	cfg := config.HostConfig{
		ID: "local",
		// Type is empty, so default branch is used
	}
	target := "my-session"
	got := AttachCommandString(cfg, target)

	want := "tmux -L jin attach -t my-session"
	if got != want {
		t.Errorf("AttachCommandString(local) = %q, want %q", got, want)
	}
}

func TestAttachCommandString_SSHNoOpts(t *testing.T) {
	cfg := config.HostConfig{
		ID:   "simple",
		Type: "ssh",
		Host: "simple-host",
	}
	target := "sess1"
	got := AttachCommandString(cfg, target)

	// Should not have extra spaces from empty SSHOpts
	if !strings.Contains(got, "-t simple-host") {
		t.Errorf("SSH command without opts should have '-t simple-host', got %q", got)
	}

	expectedRemote := "tmux -L jin attach -t sess1"
	if !strings.Contains(got, "'"+expectedRemote+"'") {
		t.Errorf("SSH command should contain quoted remote command, got %q", got)
	}
}

// --- sshAttachCommand ---

func TestSSHAttachCommand(t *testing.T) {
	t.Run("builds correct SSH command", func(t *testing.T) {
		cfg := config.HostConfig{
			ID:   "ec2",
			Type: "ssh",
			Host: "ec2-host",
		}
		cmd := sshAttachCommand(cfg, "jin:my-session")

		// cmd.Path should end with "ssh"
		if !strings.HasSuffix(cmd.Path, "ssh") {
			t.Errorf("cmd.Path should end with 'ssh', got %q", cmd.Path)
		}

		args := cmd.Args
		// Args[0] is "ssh" (the command name passed to exec.Command)
		if args[0] != "ssh" {
			t.Errorf("Args[0] should be %q, got %q", "ssh", args[0])
		}

		joined := strings.Join(args, " ")

		// Verify ControlMaster=no is present
		if !strings.Contains(joined, "ControlMaster=no") {
			t.Errorf("args should contain ControlMaster=no, got %v", args)
		}

		// Verify ControlPath references the right socket
		expectedCtrlPath := SSHControlPath("ec2")
		if !strings.Contains(joined, "ControlPath="+expectedCtrlPath) {
			t.Errorf("args should contain ControlPath=%s, got %v", expectedCtrlPath, args)
		}

		// Verify ClearAllForwardings=yes
		if !strings.Contains(joined, "ClearAllForwardings=yes") {
			t.Errorf("args should contain ClearAllForwardings=yes, got %v", args)
		}

		// Verify -t and host
		if !strings.Contains(joined, "-t ec2-host") {
			t.Errorf("args should contain '-t ec2-host', got %v", args)
		}

		// Verify remote tmux command
		if !strings.Contains(joined, "tmux -L jin attach -t jin:my-session") {
			t.Errorf("args should contain remote tmux attach command, got %v", args)
		}
	})

	t.Run("includes SSHOpts", func(t *testing.T) {
		cfg := config.HostConfig{
			ID:      "dev",
			Type:    "ssh",
			Host:    "dev-host",
			SSHOpts: []string{"-p", "2222", "-i", "/path/to/key"},
		}
		cmd := sshAttachCommand(cfg, "jin:sess1")
		joined := strings.Join(cmd.Args, " ")

		if !strings.Contains(joined, "-p 2222") {
			t.Errorf("args should contain '-p 2222', got %v", cmd.Args)
		}
		if !strings.Contains(joined, "-i /path/to/key") {
			t.Errorf("args should contain '-i /path/to/key', got %v", cmd.Args)
		}
	})
}

// --- dockerAttachCommand ---

func TestDockerAttachCommand(t *testing.T) {
	t.Run("builds correct Docker command", func(t *testing.T) {
		cfg := config.HostConfig{
			ID:        "dev",
			Type:      "docker",
			Container: "my-container",
		}
		cmd := dockerAttachCommand(cfg, "jin:my-session")

		// cmd.Path should end with "docker"
		if !strings.HasSuffix(cmd.Path, "docker") {
			t.Errorf("cmd.Path should end with 'docker', got %q", cmd.Path)
		}

		// Verify args: docker exec -it <container> tmux -L jin attach -t <target>
		expectedArgs := []string{"exec", "-it", "my-container", "tmux", "-L", "jin", "attach", "-t", "jin:my-session"}
		args := cmd.Args[1:] // skip cmd.Args[0] which is the program path

		if len(args) != len(expectedArgs) {
			t.Fatalf("got %d args %v, want %d args %v", len(args), args, len(expectedArgs), expectedArgs)
		}
		for i, want := range expectedArgs {
			if args[i] != want {
				t.Errorf("args[%d] = %q, want %q", i+1, args[i], want)
			}
		}
	})

	t.Run("uses correct container name", func(t *testing.T) {
		cfg := config.HostConfig{
			ID:        "staging",
			Type:      "docker",
			Container: "staging-claude",
		}
		cmd := dockerAttachCommand(cfg, "jin:sess-abc")
		joined := strings.Join(cmd.Args, " ")

		if !strings.Contains(joined, "staging-claude") {
			t.Errorf("args should contain container name 'staging-claude', got %v", cmd.Args)
		}
		if !strings.Contains(joined, "jin:sess-abc") {
			t.Errorf("args should contain tmux target 'jin:sess-abc', got %v", cmd.Args)
		}
	})
}

// --- AttachCommand dispatcher ---

func TestAttachCommand_SSH(t *testing.T) {
	cfg := config.HostConfig{
		ID:   "ec2",
		Type: "ssh",
		Host: "ec2-host",
	}
	cmd := AttachCommand(cfg, "jin:my-session")

	// Should produce an SSH command
	if !strings.HasSuffix(cmd.Path, "ssh") {
		t.Errorf("AttachCommand for SSH should produce ssh command, got %q", cmd.Path)
	}
	joined := strings.Join(cmd.Args, " ")
	if !strings.Contains(joined, "ec2-host") {
		t.Errorf("SSH attach command should reference host, got %v", cmd.Args)
	}
	if !strings.Contains(joined, "tmux -L jin attach -t jin:my-session") {
		t.Errorf("SSH attach command should contain remote tmux attach, got %v", cmd.Args)
	}
}

func TestAttachCommand_Docker(t *testing.T) {
	cfg := config.HostConfig{
		ID:        "dev",
		Type:      "docker",
		Container: "my-container",
	}
	cmd := AttachCommand(cfg, "jin:my-session")

	// Should produce a Docker command
	if !strings.HasSuffix(cmd.Path, "docker") {
		t.Errorf("AttachCommand for Docker should produce docker command, got %q", cmd.Path)
	}
	joined := strings.Join(cmd.Args, " ")
	if !strings.Contains(joined, "my-container") {
		t.Errorf("Docker attach command should reference container, got %v", cmd.Args)
	}
}

func TestAttachCommand_UnknownType(t *testing.T) {
	cfg := config.HostConfig{
		ID:   "unknown",
		Type: "unknown-type",
	}
	cmd := AttachCommand(cfg, "jin:my-session")

	// Unknown type falls back to local tmux command
	if !strings.HasSuffix(cmd.Path, "tmux") {
		t.Errorf("AttachCommand for unknown type should fall back to tmux, got %q", cmd.Path)
	}
	joined := strings.Join(cmd.Args, " ")
	if !strings.Contains(joined, "select-window") {
		t.Errorf("local fallback should use select-window, got %v", cmd.Args)
	}
	if !strings.Contains(joined, "-L jin") {
		t.Errorf("local fallback should use -L jin, got %v", cmd.Args)
	}
	if !strings.Contains(joined, "-t jin:my-session") {
		t.Errorf("local fallback should contain target, got %v", cmd.Args)
	}
}
