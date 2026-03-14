# IPC Protocol

## Transport

- Unix domain socket: `~/.ccvalet/run/daemon.sock`
- One request / one response per connection (no connection pooling)
- JSON encoding/decoding

## Message Format

```go
// Request (client → server)
type Request struct {
    Action  string          `json:"action"`
    Data    json.RawMessage `json:"data,omitempty"`
    Visited []string        `json:"visited,omitempty"` // Host IDs already visited (loop prevention)
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

## Bidirectional Routing

Daemons use visited-based routing to support bidirectional communication between hosts.

When `host_id` is not "local" and not the current daemon's own host ID, the request is forwarded to the target host. Before forwarding, the current daemon's `hostID` is appended to `req.Visited`. If the target `hostID` is already in `Visited`, a "routing loop detected" error is returned.

For aggregation actions (`list`, `notification-history`), the daemon queries all reachable hosts (configured remotes + peers registered via reverse tunnel), skipping any host already in `Visited`.

### Peer Registration

A peer is a daemon connected via SSH reverse tunnel (`-R`). When the master starts a slave, it passes `--peer-socket` and `--peer-id` flags. The slave uses these to register the master as a peer, enabling bidirectional communication.

### Host ID

Each daemon has a `hostID` (flag `--host-id` > config `host_id` > default `"local"`). This ID is used in `Visited` arrays and for tagging sessions/notifications.

## Adding a New Action

1. Add a case to the `handleRequest()` switch in `server.go`
2. Define a Request type if needed
3. Implement a `handle{Action}()` method
4. Add a corresponding method in `client.go`
5. Add a CLI command in `cmd/ccvalet/cmd/` (→ docs/adding-commands.md)
