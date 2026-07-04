# Session Lifecycle

## Status State Machine

```
                    CreateWithOptions()
                          │
                          ▼
                     StatusStopped
                          │
                   StartBackground()
                          │
                          ▼
                    StatusRunning ◄─── RecoverTmuxSessions()
                     │    │    │
    UserPromptSubmit │    │    │ Notification(permission_prompt)
                     ▼    │    ▼
              StatusThinking  StatusPermission
                     │    │    │
                Stop │    │    │ Stop
                     ▼    ▼    ▼
                    StatusIdle
                          │
              pane dead / Kill()
                          │
                          ▼
                    StatusStopped
```

Status constants (session/session.go):
- `creating`   - CC starting up (currently unused, reserved)
- `stopped`    - Process stopped
- `running`    - Running (initial state before any hook is received)
- `idle`       - Waiting for input (Stop hook)
- `thinking`   - Processing (UserPromptSubmit hook)
- `permission` - Waiting for permission (Notification hook)

## Session Structure

```go
Session (persisted)
├─ ID              string    // UUID (compatible with Claude Code --session-id)
├─ Name            string    // Display name (default: basename of WorkDir)
├─ WorkDir         string    // Working directory (dynamically updated via hook cwd)
├─ CreatedAt       time.Time // Creation timestamp
├─ Status          Status
├─ LastActiveAt    time.Time
├─ ErrorMessage    string    // Error message (e.g., on startup failure)
├─ ClaudeSessionID string    // Claude Code session ID
├─ ClaudeSessionStarted bool // Used to determine --resume vs --session-id
├─ HostID          string    // "local" or remote host name
├─ TmuxWindowName  string    // Inner tmux session name
└─ TmuxPaneID      string    // CC pane ID (e.g., "%42")

Session (runtime only, json:"-")
├─ LastOutputTime  time.Time // For idle stability detection
├─ StartedAt      time.Time // Prevents false error detection right after startup
├─ SSHAuthSock    string    // For git operations
├─ CurrentWorkDir string    // tmux pane_current_path
├─ CurrentBranch  string    // git branch
└─ IsGitRepo      bool
```

## Creation Flow

1. `Manager.CreateWithOptions()` creates a Session and persists it via Store
2. `Manager.StartBackground()` → `startSession()` → `startSessionTmux()`
3. `ensureTmuxClient()` initializes the inner tmux (`-L jin`)
4. `ensureClaudeTrustState()` sets trust config in `~/.claude/settings.local.json`
5. Creates an inner tmux session and runs `claude --session-id {ID}`
6. `TagManagedPane()` tags the pane for remain-on-exit
7. Starts `captureOutputTmux()` goroutine for polling

## Recovery (On Daemon Restart)

`RecoverTmuxSessions()`:
1. Loads all persisted sessions (initialized as Status=Stopped)
2. For sessions with TmuxWindowName, checks if the inner tmux is alive
3. Alive → StatusRunning + restart `captureOutputTmux()`
4. Pane dead → StatusStopped (TmuxWindowName preserved for RespawnPane)
5. Session itself gone → Clear TmuxWindowName + StatusStopped

## Auto-Recovery on Resume Failure

Inside `captureOutputTmux()`, detects pane death within 10 seconds of startup:
1. Determines that `claude --resume` has failed
2. Generates a new ClaudeSessionID
3. Respawns pane with `claude --session-id {newID}`
4. If successful, continues as a new session

## WorkDir Tracking

WorkDir is updated through two paths:
1. **Via Hook**: `HandleHookEvent()` `cwd` field (Claude Code's actual CWD)
2. **Via Polling**: `captureOutputTmux()` `GetPaneCurrentPath()` (tmux pane's CWD)
