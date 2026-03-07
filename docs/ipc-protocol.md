# IPC Protocol

## Transport

- Unix domain socket: `~/.ccvalet/run/daemon.sock`
- 1リクエスト/1レスポンスで接続クローズ（コネクションプーリングなし）
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

| Action | Data Type | 説明 |
|--------|-----------|------|
| `new` | `NewRequest` | セッション作成 |
| `list` | (なし) | 全セッション一覧 |
| `start` | `IDRequest` | セッション開始 |
| `kill` | `IDRequest` | セッション終了 |
| `delete` | `IDRequest` | セッション削除 |
| `stop` | (なし) | デーモン停止 |
| `list-hosts` | (なし) | ホスト一覧 |
| `hook` | `HookRequest` | Claude Code hookイベント |
| `notification-history` | (なし) | 通知履歴取得 |

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

`host_id` が "local" 以外の場合、Masterデーモンが該当Slaveへリクエストを転送する。
転送時に `host_id` フィールドを除去し、Slaveが再帰転送しないようにする。

## 新規Action追加パターン

1. `server.go` の `handleRequest()` switch に case 追加
2. 必要に応じて Request 型を定義
3. `handle{Action}()` メソッドを実装
4. `client.go` に対応するメソッド追加
5. `cmd/ccvalet/cmd/` にCLIコマンド追加（→ docs/adding-commands.md）
