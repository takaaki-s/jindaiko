# ccvalet

Claude Codeの複数セッションをtmux TUIで管理するCLIツール。

## Build & Test

```
make build       # → bin/ccvalet
make test        # go test -v ./...
make fmt         # go fmt ./...
make lint        # go vet ./...
make install     # go install ./cmd/ccvalet
```

## Project Layout

```
cmd/ccvalet/cmd/     Cobra CLIコマンド (root, daemon, session, tui, hook, ...)
internal/
  config/            Viper設定管理 (~/.ccvalet/config.yaml)
  daemon/            Unix socket IPCサーバー/クライアント
  session/           セッション管理 (コアドメイン, 最大モジュール)
  tui/               BubbleTea TUI (最大コード量)
  tmux/              tmux -L ccvalet セッション制御
  host/              マルチホスト管理 (SSH/Docker)
  tunnel/            SSHトンネルライフサイクル
  notify/            デスクトップ通知 (macOS/Linux)
  transcript/        Claude Code transcript読取 (~/.claude/projects/)
```

## Docs

詳細は各ファイル参照:

- [docs/architecture.md](docs/architecture.md) — アーキテクチャ・依存関係・データフロー
- [docs/conventions.md](docs/conventions.md) — コーディング規約・パターン
- [docs/session-lifecycle.md](docs/session-lifecycle.md) — セッション状態遷移・作成・復旧
- [docs/ipc-protocol.md](docs/ipc-protocol.md) — IPCプロトコル仕様・Action一覧
- [docs/tui-guide.md](docs/tui-guide.md) — TUI開発ガイド・ビュー追加手順
- [docs/adding-commands.md](docs/adding-commands.md) — 新規CLIコマンド追加手順
- [docs/gotchas.md](docs/gotchas.md) — 既知の落とし穴・注意事項

## Debug

```
CCVALET_DEBUG=1 ccvalet daemon start
```

ログ: `~/.ccvalet/daemon-debug.log`, `~/.ccvalet/hook-debug.log`

## Key Dependencies

Go 1.24.5 / cobra (CLI) / bubbletea (TUI) / viper (config) / lipgloss (styling)

## Data Directories

```
~/.ccvalet/
  config.yaml          ユーザー設定
  state.yaml           永続状態
  sessions/{uuid}.json セッションデータ
  run/daemon.sock      IPC Unix socket
```

## Claude Code Hooks

`~/.claude/settings.json` に設定:
- `UserPromptSubmit` → セッションを "thinking" に
- `Stop` → "idle" に + タスク完了通知
- `Notification` → "permission" に + 許可待ち通知

詳細は README.md の「Claude Code Hooks設定」セクション参照。

## Testing

テストファイルは未整備。新規コードには `_test.go` を追加すること。
