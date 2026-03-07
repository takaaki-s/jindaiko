# Architecture

## Overview

ccvaletはClaude Codeの複数セッションをtmux上で管理するCLI/TUIツール。
デーモンプロセスがUnix socketでIPCを提供し、CLIとTUIがクライアントとして接続する。

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

## Hook Flow (状態検出)

```
Claude Code (hook event)
  → ccvalet hook (cmd/ccvalet/cmd/hook.go)
    → daemon.Client.Send("hook", HookRequest)
      → daemon.Server.handleHook()
        → session.Manager.HandleHookEvent()
          → Session.Status 更新 + Store.Save + notify
```

Hook Events:
- `UserPromptSubmit` → StatusThinking
- `Stop` → StatusIdle + タスク完了通知
- `Notification(permission_prompt)` → StatusPermission + 許可待ち通知

## Package Dependency

```
cmd/ccvalet/cmd/   → daemon (client), config, session (types only)
                      │
daemon/            → session, config, host, notify, tmux, tunnel
                      │
session/           → config, tmux, notify, transcript, worktree
                      │
host/              → config, tunnel, daemon (client)
                      │
tui/               → daemon (client), config, session (Info type)
                      │
config/            → (external: viper)
tmux/              → (external: tmux CLI)
tunnel/            → (external: ssh, docker CLI)
notify/            → (external: OS notification)
transcript/        → (file I/O: ~/.claude/projects/)
worktree/          → (external: git CLI)
```

configは多くのパッケージから参照される基盤パッケージ。
sessionパッケージがコアドメインで最大のビジネスロジックを持つ。

## Remote Architecture

ローカルデーモン(Master)がリモートホスト上のデーモン(Slave)をSSHトンネル経由で制御する。

```
Master daemon (local)
  ├─ host.Registry     ... ホスト一覧管理
  ├─ tunnel.Manager    ... SSH/Dockerトンネルのライフサイクル
  └─ RemoteClient      ... Slave daemonへのIPC転送
       │ (SSH tunnel / Docker exec)
       ▼
Slave daemon (remote)
  ├─ session.Manager
  └─ tmux.Client
```

リモート宛リクエストは `forwardToSlave()` でhost_idを除去して転送される。

## File Storage

```
~/.ccvalet/
  ├─ config.yaml            ... ユーザー設定 (keybindings, hosts)
  ├─ state.yaml             ... 永続状態 (StateManager)
  ├─ sessions/
  │   └─ {uuid}.json        ... セッション永続化データ
  ├─ run/
  │   └─ daemon.sock        ... Unix domain socket
  ├─ daemon-debug.log       ... デーモンデバッグログ
  └─ hook-debug.log         ... hookデバッグログ
```
