# Architecture

## Overview

ccvalet is a CLI/TUI tool that manages multiple Claude Code sessions on tmux.
A daemon process provides IPC via Unix socket, with CLI and TUI connecting as clients.

## Data Flow

```
User
 ├─ ccvalet tui          ─┐
 ├─ ccvalet session new   │  daemon.Client (Unix socket)
 ├─ ccvalet session list  ├─────────────────────────────→ daemon.Server
 └─ ccvalet session kill ─┘                                   │
                                                              ▼
                                                      session.Manager
                                                         │       │
                                                         ▼       ▼
                                                    tmux.Client  Store (JSON)
                                                         │
                                                         ▼
                                                   tmux -L ccvalet
                                                    (inner tmux)
                                                         │
                                                         ▼
                                                   claude / claude --resume
```

## Hook Flow (State Detection)

```
Claude Code (hook event)
  → ccvalet hook (cmd/ccvalet/cmd/hook.go)
    → daemon.Client.Send("hook", HookRequest)
      → daemon.Server.handleHook()
        → session.Manager.HandleHookEvent()
          → Session.Status update + Store.Save + notify
```

Hook Events:
- `UserPromptSubmit` → StatusThinking
- `Stop` → StatusIdle + task completion notification
- `Notification(permission_prompt)` → StatusPermission + permission-waiting notification

## Package Dependency

```
cmd/ccvalet/cmd/   → daemon (client), config, session (types only), tui, tmux, host
                      │
daemon/            → session, config, host, notify, tmux, tunnel
                      │
session/           → config, tmux, notify, transcript
                      │
host/              → config, notify, session
                      │
tui/               → daemon (client), config, session (Info type), host
                      │
config/            → (external: viper)
tmux/              → (external: tmux CLI)
tunnel/            → (external: ssh, docker CLI)
notify/            → (external: OS notification)
transcript/        → (file I/O: ~/.claude/projects/)
```

config is a foundational package referenced by many others.
The session package is the core domain with the most business logic.

## Remote Architecture

The local daemon (Master) controls daemons on remote hosts (Slave) via SSH tunnels.

```
Master daemon (local)
  ├─ host.Registry     ... Host list management
  ├─ tunnel.Manager    ... SSH/Docker tunnel lifecycle
  └─ RemoteClient      ... IPC forwarding to Slave daemon
       │ (SSH tunnel / Docker exec)
       ▼
Slave daemon (remote)
  ├─ session.Manager
  └─ tmux.Client
```

Requests destined for remote hosts are forwarded via `forwardToSlave()`, which strips the host_id before forwarding.

## File Storage

```
~/.ccvalet/
  ├─ config.yaml            ... User settings (keybindings, hosts)
  ├─ state.yaml             ... Persistent state (StateManager)
  ├─ sessions/
  │   └─ {uuid}.json        ... Session persistence data
  ├─ run/
  │   └─ daemon.sock        ... Unix domain socket
  ├─ daemon-debug.log       ... Daemon debug log
  └─ hook-debug.log         ... Hook debug log
```
