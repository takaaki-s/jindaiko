# Architecture

## Overview

honjin is a CLI/TUI tool that manages multiple interactive agent sessions on
tmux — Claude Code is the first-class citizen and, at present, the only
adapter shipped in-tree; other adapters (Codex CLI, Aider, …) plug in via
`internal/agent/<kind>/`. A daemon process provides IPC via Unix socket,
with CLI and TUI connecting as clients.

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
          → agent.StatusSource.Interpret()  ── adapter-owned event→status mapping
          → Session.Status update + Store.Save + notify
```

The event vocabulary (which hook name means what status) lives entirely in
the agent adapter — `HandleHookEvent` itself is agent-agnostic wiring. The
Claude Code mapping is documented in `internal/agent/claude/status.go`; the
common cases are:

- `UserPromptSubmit` → StatusThinking + Layer C Description upgrade attempt
- `Stop` → StatusIdle + task-complete notification
- `Notification(permission_prompt)` → StatusPermission + permission notification

## Agent Adapters

Every session records an `AgentKind` (`"claude"` today; more later). The
Manager fetches the concrete adapter through the `session.AgentResolver`
interface:

```
internal/session/       owns Agent + AgentResolver interfaces (no import of agent/)
internal/agent/         re-exports the same types as aliases; hosts registry
internal/agent/register blank-import → wires every kind into the registry at init()
internal/agent/claude/  Claude Code adapter: SpawnCommand / StatusSource / Setup
                         (hooks-settings.json + trust-dialog files), Description
                         (Layer C enhancer over the transcript)
```

The daemon injects a thin resolver (`agent.Lookup`) into `session.Manager`,
so `internal/session/` never imports `internal/agent/*`. Adding a new
adapter is a matter of dropping `internal/agent/<kind>/` with an
implementation and adding one line to `internal/agent/register/register.go`.

## Session Description Model

Sessions carry a human-readable `Description` field decoupled from the technical `ID`. It is filled by a 3-layer generation pipeline (see [session-lifecycle.md](session-lifecycle.md) for the state machine):

- **Layer A (baseline)** — Always populated at creation from `<repo>[:<branch>][:<subpath>]`, or `<main-repo>:<worktree-name>` for worktree sessions. Agent-independent. Implemented in `internal/session/description.go`.
- **Layer B (manual)** — CLI `--description` on `jin session new` and the `jin session set-description` subcommand. Sets `DescriptionLocked = true` to freeze the value against auto-upgrade. (The TUI create form intentionally does not expose a manual description step: Layer A + Layer C cover the common case; users who need a manual label use the CLI paths.)
- **Layer C (agent-specific enhancer)** — Returned by the adapter through `Agent.Description() DescriptionEnhancer`; `HandleHookEvent` looks it up from the resolved adapter on every `UserPromptSubmit` / `Stop` hook (Manager holds no separate enhancer field). The Claude Code implementation (`internal/agent/claude/`) reads the first user turn from the transcript and upgrades the Description if the session is unlocked and still at the baseline. `UserPromptSubmit` is the fast path (fires as soon as CC records the prompt); `Stop` is the reliable fallback that runs after the transcript has been flushed. Future agents (Codex, Aider, …) plug in the same way — return an enhancer from `Description()` and the plumbing works for them unchanged.

The `Name` field was retired in favour of `Description` + `DescriptionLocked`. Existing session JSON is migrated in place on daemon startup by `internal/session/migration.go`.

## Package Dependency

```
cmd/jin/cmd/       → daemon (client), config, session (types only), tui, tmux,
                       _ agent/register (blank import so kinds are registered)
                      │
daemon/            → session, config, notify, tmux, agent (registry Lookup)
                      │
session/           → config, tmux, notify, transcript   (no import of agent/*)
                      │
agent/             → session (borrows Agent + supporting types via aliases)
agent/claude/      → agent, session, transcript, debug   (CC-specific adapter)
agent/register/    → agent, agent/claude (init-time Register)
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
