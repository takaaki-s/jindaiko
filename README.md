**English** | [日本語](README.ja.md)

# honjin

A CLI tool for running and managing multiple Claude Code sessions simultaneously.

https://github.com/user-attachments/assets/62e9d64a-aa7d-42f8-8edf-03f724fe0ee4

## Features

- **Multi-session management**: Run multiple Claude Code sessions in the background simultaneously
- **TUI**: Interactive terminal UI for listing, monitoring, and operating sessions
- **Attach/Detach**: Quickly switch between sessions (`Ctrl+]` to detach)
- **Real-time status tracking**: Live display of working directory, branch, and latest message
- **Search & Paging**: Incremental search by session name, directory, or branch
- **Desktop notifications**: OS notifications for permission requests and task completion (macOS / Linux)
- **Remote host support**: Manage remote sessions via SSH / Docker

## Installation

### Download from GitHub Releases

Download the binary for your OS/architecture from the [Releases page](https://github.com/takaaki-s/honjin/releases).

```bash
# Example: Linux amd64
curl -Lo honjin.tar.gz https://github.com/takaaki-s/honjin/releases/latest/download/honjin_0.1.0_linux_amd64.tar.gz
tar xzf honjin.tar.gz
sudo mv jin /usr/local/bin/
```

### Go install

```bash
go install github.com/takaaki-s/honjin/cmd/jin@latest
```

### Build from source

```bash
git clone https://github.com/takaaki-s/honjin.git
cd honjin
make build    # Build to bin/jin
make install  # Install to $GOPATH/bin
```

## Quick Start

### 1. Start the daemon

```bash
jin daemon start
```

### 2. Launch the TUI

```bash
jin ui
```

### 3. Create and attach to a session

Press `n` in the TUI to create a session, then `Enter` to attach.

Press `Ctrl+]` to detach and return to the TUI.

## Session Status

Session states are detected via Claude Code [hooks](https://docs.anthropic.com/en/docs/claude-code/hooks) in an event-driven manner.

| Status | Icon | Detection | Description |
|--------|------|-----------|-------------|
| `thinking` | ⚡ | `UserPromptSubmit` hook | Processing |
| `permission` | ? | `Notification` hook | Awaiting permission |
| `running` | ▶ | Internal | Running |
| `creating` | + | Internal | Creating (CC starting up) |
| `idle` | ○ | `Stop` hook | Waiting for input |
| `stopped` | ■ | Process death detection | Stopped |

## CLI Commands

### Daemon management

```bash
jin daemon start   # Start daemon
jin daemon stop    # Stop daemon
jin daemon status  # Check status
```

### Session management

```bash
# Create session (interactive via TUI - recommended)
jin session new

# Create session (specify working directory)
jin session new --workdir ~/repos/myrepo

# List sessions
jin session list

# List sessions in JSON format (for scripting / LLM integration)
jin session list --json

# Attach to a session
jin session attach <session-name>

# Get session details
jin session info <session-name>

# Send a prompt to a session
jin session send <session-name> "your prompt here"

# Wait for a session to become idle (default timeout: 300s)
jin session wait <session-name>
jin session wait <session-name> --timeout 60

# Get the last assistant message
jin session output <session-name>

# Get the last N conversation pairs
jin session output <session-name> --last 3

# Kill a session
jin session kill <session-name>

# Delete a session
jin session delete <session-name>

# Bulk delete stopped sessions
jin cleanup stopped
jin cleanup stopped --dry-run   # Preview what will be deleted
```

> **Aliases**: `session` can be shortened to `sess` (e.g., `jin sess list`). `list` to `ls`, `delete` to `rm`.

### LLM API (scripting / automation)

The following commands support `--json` for structured output, enabling integration with scripts and other LLM agents.

```bash
# All session commands support --json
jin session list --json
jin session new --workdir ~/repos/myrepo --json
jin session info <session-name> --json
jin session kill <session-name> --json

# Send a prompt and wait for completion
jin session send <session-name> "fix the failing test" --json
jin session wait <session-name> --timeout 120 --json
jin session output <session-name> --json

# Pipeline example: send a prompt, wait, get output
jin session send my-session "refactor main.go"
jin session wait my-session --timeout 300
jin session output my-session --last 1
```

#### Orchestration: parent Claude driving child sessions

For richer orchestration (e.g. a parent Claude that needs to inspect what a
child actually *did*, not just the assistant's text), use `session result`.
It returns structured `tool_use` / `tool_result` / `thinking` blocks parsed
directly from the Claude Code transcript JSONL — no tmux pane scraping, no
truncation by scrollback buffer.

```bash
# Send a prompt, wait until the child stops (idle OR waiting on permission),
# then fetch what it actually did.
jin session prompt my-session "run go test ./... and report failures"
jin session wait my-session --until idle,permission --timeout 600
jin session result my-session --json | jq '.entries[].blocks[] | select(.kind=="tool_result")'

# Incremental fetch: only entries after a checkpoint.
# --since is strictly exclusive: pass the last entry's timestamp to receive
# only entries that came after it (no duplicates). Claude Code emits timestamps
# with millisecond precision (e.g. "2026-04-09T13:23:10.456Z").
T1=$(jin session result my-session --json | jq -r '.entries[-1].timestamp')
jin session prompt my-session "now also run go vet"
jin session wait my-session --until idle,permission
jin session result my-session --since "$T1" --json

# Filter to a specific tool, or to errors only
jin session result my-session --tool Bash --json
jin session result my-session --errors-only --json
```

`prompt` is an alias for `send`. All flags work with `--host` for remote/peer sessions.

#### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Session not found |
| 3 | Daemon not running |
| 4 | Timeout (`session wait`) |

### Utilities

```bash
jin session workdir <session-name>    # Print session's working directory path
jin session edit <session-name>       # Open session's working directory in EDITOR
```

> **Note**: `workdir` / `edit` only work correctly for local sessions (host type `local`).

The following shell functions are useful:

```bash
# cd to a session's working directory
cc-cd() { cd "$(jin session workdir "$1")"; }

# Select a session with fzf and cd to its working directory
cc-cdf() {
  local session
  session=$(jin session list | tail -n +2 | fzf --height 40% --reverse | awk '{print $1}')
  [[ -n "$session" ]] && cd "$(jin session workdir "$session")"
}

# Select a session with fzf and attach
cc-attach() {
  local session
  session=$(jin session list | tail -n +2 | fzf --height 40% --reverse | awk '{print $1}')
  [[ -n "$session" ]] && jin session attach "$session"
}
```

### Shell Completion

```bash
# bash
source <(jin completion bash)

# zsh
source <(jin completion zsh)

# fish
jin completion fish | source
```

## Configuration

honjin follows the [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/). Files are split across config / state / runtime directories:

```
$XDG_CONFIG_HOME/honjin/      (default: ~/.config/honjin)
└── config.yaml                # Configuration file

$XDG_STATE_HOME/honjin/       (default: ~/.local/state/honjin)
├── state.yaml                 # State file (last used repository, etc.)
├── sessions/                  # Session data
├── hooks-settings.json        # Generated hooks settings (auto-managed)
├── daemon-debug.log           # Daemon debug log (when JIN_DEBUG=1)
└── hook-debug.log             # Hook debug log (when JIN_DEBUG=1)

$XDG_RUNTIME_DIR/honjin/      (fallback: $TMPDIR/honjin-<uid>)
└── daemon.sock                # Daemon socket
```

### Example configuration (`~/.config/honjin/config.yaml`)

```yaml
# Customize keybindings (defaults are used when omitted)
keybindings:
  # Session list view
  up: ["up", "k"]
  down: ["down", "j"]
  attach: ["enter"]
  new: ["n"]
  kill: ["x"]
  delete: ["d"]
  refresh: ["r"]
  search: ["/"]
  vscode: ["v"]
  notifications: ["!"]
  quit: ["q", "ctrl+c"]
  help: ["?"]
  # Session creation form
  next_field: ["tab"]
  prev_field: ["shift+tab"]
  submit: ["enter"]
  cancel_form: ["esc"]
  # While attached
  detach: ["ctrl+]"]  # Default: ctrl+]
                       # Supported keys: ctrl+^, ctrl+], ctrl+\, ctrl+g
```

## TUI Keybindings

### Session list view

| Key | Action |
|-----|--------|
| `↑/k` | Move up |
| `↓/j` | Move down |
| `←/h` | Previous page |
| `→/l` | Next page |
| `/` | Search sessions (name, directory, branch) |
| `Enter` | Attach to session |
| `n` | Create new session |
| `x` | Kill session |
| `d` | Delete session |
| `r` | Refresh list |
| `v` | Open in VS Code |
| `!` | Notification history |
| `?` | Show help |
| `q` | Quit |

### Session creation form

| Key | Action |
|-----|--------|
| `Tab` | Move to next field |
| `Shift+Tab` | Move to previous field |
| `Enter` | Create session |
| `Esc` | Cancel |

While attached, press `Ctrl+]` (default) to detach and return to the TUI.
You can change this in `config.yaml` under `keybindings.detach`.

Supported detach keys:

| Key | Description |
|-----|-------------|
| `ctrl+]` | Default |
| `ctrl+^` | Ctrl+Shift+6 |
| `ctrl+\` | Ctrl+Backslash |
| `ctrl+g` | Ctrl+G |

## Claude Code Hooks

honjin uses Claude Code hooks to detect session state changes. **Hooks are configured automatically** — no manual setup required.

When a session starts, honjin generates `$XDG_STATE_HOME/honjin/hooks-settings.json` (default `~/.local/state/honjin/hooks-settings.json`) and passes it to Claude Code via `claude --settings`. This file wires up the following hooks:

| Hook Event | Role |
|-----------|------|
| `UserPromptSubmit` | User submits a prompt → set session to `thinking` |
| `PostToolUse` | Tool execution ends → set session to `thinking` (recovers from `permission` state) |
| `Stop` | Claude's turn ends → set session to `idle` (send task completion notification) |
| `Notification` | Permission request, etc. → set session to `permission` (send permission request notification) |

## Remote Hosts (SSH / Docker)

In addition to local sessions, you can manage Claude Code sessions running on remote SSH hosts or Docker containers.

### Architecture

The Master daemon on your local machine communicates with Slave daemons on remote hosts via SSH tunnels (or Docker volume mounts). The Slave runs the same `jin daemon` binary.

Communication is **bidirectional**: the Master establishes a forward tunnel (`-L`) to reach the Slave, and a reverse tunnel (`-R`) so the Slave can reach the Master back. This allows either side to act as an orchestrator. A `visited` array in each request prevents routing loops.

Each daemon has a **host ID** (default: `"local"`). You can set it in `config.yaml` or via the `--host-id` flag:

```yaml
# ~/.config/honjin/config.yaml
host_id: mac   # Identifies this daemon in bidirectional routing
hosts:
  - id: my-server
    type: ssh
    host: my-remote-host
```

### SSH Remote Prerequisites

The SSH tunnel uses Unix socket forwarding (`-L`). Ensure the remote sshd allows it:

```bash
# Check on the remote host
sudo sshd -T | grep allowtcpforwarding
```

If the output is `allowtcpforwarding no`, add to `/etc/ssh/sshd_config`:

```
AllowTcpForwarding local
```

Then restart sshd:

```bash
sudo systemctl restart sshd
```

> **Note:** `AllowStreamLocalForwarding yes` alone is not sufficient — `AllowTcpForwarding` controls Unix socket `-L` forwarding as well, regardless of the OpenSSH version.

### SSH Remote Setup

**1. Install jin and tmux on the remote host (first time only)**

```bash
# Log in to remote host
ssh my-remote-host

# Install jin
go install github.com/takaaki-s/honjin/cmd/jin@latest

# Install tmux (if not already installed)
sudo apt install -y tmux  # Ubuntu/Debian
```

**2. Add host configuration to config.yaml on your local machine**

```yaml
hosts:
  - id: my-server
    type: ssh
    host: my-remote-host
    ssh_opts:          # SSH connection optimization (recommended)
      - "-o"
      - "ControlMaster=auto"
      - "-o"
      - "ControlPath=~/.ssh/sockets/%r@%h-%p"
      - "-o"
      - "ControlPersist=600"
```

**3. Start Master to auto-connect**

```bash
jin daemon start  # Auto-start Slave + establish tunnel
jin ui            # Manage local + remote sessions in one TUI
```

The Master automatically starts the Slave daemon on the remote host via SSH. An error message is displayed if jin is not installed on the remote host.

#### Specifying the remote jin binary path

By default, `jin` is resolved from the remote shell's `PATH`. If the binary is installed in a non-standard location (e.g., `~/.local/bin`) and is not available in non-interactive SSH sessions, specify the full path explicitly:

```yaml
hosts:
  - id: my-server
    type: ssh
    host: my-remote-host
    jin_path: /home/user/.local/bin/jin  # full path to jin on remote
```

> **Note**: SSH sessions are non-interactive, so `.bashrc` / `.zshrc` are not sourced. If jin is installed via `go install` or to `~/.local/bin` and PATH is only configured in those files, use `jin_path` or add the path to `~/.bash_profile` / `~/.profile` instead.

### Docker Setup

**1. Include jin and tmux in the container**

```dockerfile
# Add to Dockerfile
RUN apt-get update && apt-get install -y tmux
RUN go install github.com/takaaki-s/honjin/cmd/jin@latest
```

**2. Start the container with a volume mount for socket sharing**

The local socket path is automatically computed as `/tmp/jin-tunnels/{hostID}/daemon.sock` (same convention as SSH). The volume mount maps this directory to the socket directory inside the container.

```bash
# Root user
docker run -v /tmp/jin-tunnels/docker-dev:/root/.local/state/honjin my-image

# Non-root user (app)
docker run -v /tmp/jin-tunnels/docker-dev:/home/app/.local/state/honjin my-image

# Override socket_path
docker run -v /tmp/jin-tunnels/docker-dev:/var/run/honjin my-image
```

**3. Add host configuration to config.yaml on your local machine**

```yaml
hosts:
  # Basic setup (default socket path: ~/.local/state/honjin/daemon.sock)
  - id: docker-dev
    type: docker
    container: my-container

  # socket_path override (specify path inside container)
  - id: docker-ci
    type: docker
    container: ci-runner
    socket_path: /var/run/honjin/daemon.sock

  # jin_path override (if binary is not in default PATH)
  - id: docker-custom
    type: docker
    container: my-container
    jin_path: /usr/local/bin/jin
```

`socket_path` specifies the socket path inside the container (remote side). Defaults to `~/.local/state/honjin/daemon.sock` when omitted.

`jin_path` specifies the full path to the jin binary inside the container. Defaults to `jin` (resolved from PATH) when omitted.

**4. Start Master**

```bash
jin daemon start  # Auto-start Slave via docker exec
jin ui
```

> **Note**: If you recreate the container (`docker rm`), jin will be lost. Include it in the Dockerfile or persist the binary via volume mount.

## Debugging

```bash
# Enable debug logging
export JIN_DEBUG=1

# Start daemon
jin daemon start

# View logs
tail -f ~/.local/state/honjin/daemon-debug.log
```

## Requirements

- Go 1.24.5+
- tmux 3.3+
- Claude Code CLI installed

## License

MIT
