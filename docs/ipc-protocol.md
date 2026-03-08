# IPC Protocol

## Transport

- Unix domain socket: `~/.ccvalet/run/daemon.sock`
- One request / one response per connection (no connection pooling)
- JSON encoding/decoding

## Message Format

```go
// Request (client → server)
type Request struct {
    Action string          `json:"action"`
    Data   json.RawMessage `json:"data,omitempty"`
}

// Response (server → client)
type Response struct {
    Success bool            `json:"success"`
    Data    json.RawMessage `json:"data,omitempty"`
    Error   string          `json:"error,omitempty"`
}
```

## Actions

| Action | Data Type | Description |
|--------|-----------|-------------|
| `new` | `NewRequest` | Create session |
| `list` | (none) | List all sessions |
| `start` | `IDRequest` | Start session |
| `kill` | `IDRequest` | Kill session |
| `delete` | `IDRequest` | Delete session |
| `stop` | (none) | Stop daemon |
| `list-hosts` | (none) | List hosts |
| `hook` | `HookRequest` | Claude Code hook event |
| `notification-history` | (none) | Get notification history |

## Request Types

```go
type NewRequest struct {
    Name        string `json:"name"`
    WorkDir     string `json:"work_dir"`
    Start       bool   `json:"start"`
    HostID      string `json:"host_id,omitempty"`
    SSHAuthSock string `json:"ssh_auth_sock,omitempty"`
}

type IDRequest struct {
    ID     string `json:"id"`
    HostID string `json:"host_id,omitempty"`
}

type HookRequest struct {
    SessionID        string `json:"session_id"`
    CcvaletSessionID string `json:"ccvalet_session_id,omitempty"`
    HookEventName    string `json:"hook_event_name"`
    NotificationType string `json:"notification_type,omitempty"`
    CWD              string `json:"cwd,omitempty"`
}
```

## Remote Forwarding

When `host_id` is not "local", the Master daemon forwards the request to the corresponding Slave.
The `host_id` field is stripped before forwarding to prevent recursive forwarding by the Slave.

## Adding a New Action

1. Add a case to the `handleRequest()` switch in `server.go`
2. Define a Request type if needed
3. Implement a `handle{Action}()` method
4. Add a corresponding method in `client.go`
5. Add a CLI command in `cmd/ccvalet/cmd/` (→ docs/adding-commands.md)
