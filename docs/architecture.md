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
- `UserPromptSubmit` → StatusThinking + Layer C Description upgrade attempt (see below)
- `Stop` → StatusIdle + task completion notification
- `Notification(permission_prompt)` → StatusPermission + permission-waiting notification

## Session Description Model

Sessions carry a human-readable `Description` field decoupled from the technical `ID`. It is filled by a 3-layer generation pipeline (see [session-lifecycle.md](session-lifecycle.md) for the state machine):

- **Layer A (baseline)** — Always populated at creation from `<repo>[:<branch>][:<subpath>]`, or `<main-repo>:<worktree-name>` for worktree sessions. Agent-independent. Implemented in `internal/session/description.go`.
- **Layer B (manual)** — CLI `--description` on `jin session new` and the `jin session set-description` subcommand. Sets `DescriptionLocked = true` to freeze the value against auto-upgrade. (The TUI create form intentionally does not expose a manual description step: Layer A + Layer C cover the common case; users who need a manual label use the CLI paths.)
- **Layer C (agent-specific enhancer)** — Registered per-agent via `Manager.SetDescriptionEnhancer(DescriptionEnhancer)`. Currently only the Claude Code implementation (`internal/agent/claude/`) is wired in; it reads the first user turn from the transcript and upgrades the Description if the session is unlocked and still at the baseline. Fires from `HandleHookEvent` on both `UserPromptSubmit` (fast path when the transcript is already flushed) and `Stop` (reliable path after Claude Code finishes responding). Future agents (Codex, Aider, …) will register their own enhancers under `internal/agent/<name>/` — see the planned `feat/agent-abstraction` task.

The `Name` field was retired in favour of `Description` + `DescriptionLocked`. Existing session JSON is migrated in place on daemon startup by `internal/session/migration.go`.

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
