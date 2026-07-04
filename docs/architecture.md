# Architecture

## Overview

honjin is a CLI/TUI tool that manages multiple Claude Code sessions on tmux.
A daemon process provides IPC via Unix socket, with CLI and TUI connecting as clients.

## Data Flow

```
User
 ├─ jin tui              ─┐
 ├─ jin session new       │  daemon.Client (Unix socket)
 ├─ jin session list      ├─────────────────────────────→ daemon.Server
 └─ jin session kill     ─┘                                   │
                                                              ▼
                                                      session.Manager
                                                         │       │
                                                         ▼       ▼
                                                    tmux.Client  Store (JSON)
                                                         │
                                                         ▼
                                                   tmux -L jin
                                                    (inner tmux)
                                                         │
                                                         ▼
                                                   claude / claude --resume
```

## Hook Flow (State Detection)

```
Claude Code (hook event)
  → jin hook (cmd/jin/cmd/hook.go)
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
cmd/jin/cmd/       → daemon (client), config, session (types only), tui, tmux
                      │
daemon/            → session, config, notify, tmux
                      │
session/           → config, tmux, notify, transcript
                      │
tui/               → daemon (client), config, session (Info type)
                      │
config/            → (external: viper)
tmux/              → (external: tmux CLI)
notify/            → (external: OS notification)
transcript/        → (file I/O: ~/.claude/projects/)
```

config is a foundational package referenced by many others.
The session package is the core domain with the most business logic.

## File Storage

honjin follows the [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/):

```
$XDG_CONFIG_HOME/honjin/        (default: ~/.config/honjin)
  └─ config.yaml                 ... User settings (keybindings)

$XDG_STATE_HOME/honjin/         (default: ~/.local/state/honjin)
  ├─ state.yaml                  ... Persistent state (StateManager)
  ├─ sessions/
  │   └─ {uuid}.json             ... Session persistence data
  ├─ hooks-settings.json         ... Generated Claude Code hooks settings
  ├─ daemon-debug.log            ... Daemon debug log
  └─ hook-debug.log              ... Hook debug log

$XDG_RUNTIME_DIR/honjin/        (fallback: $TMPDIR/honjin-<uid>)
  └─ daemon.sock                 ... Unix domain socket
```
